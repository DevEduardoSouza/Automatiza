import { ROUTES } from '@/constants/routes';
import {
  AuthService,
  ChangePasswordRequestSchema,
  ForgotPasswordRequestSchema,
  GetCurrentUserRequestSchema,
  ListSessionsRequestSchema,
  LoginRequestSchema,
  LogoutRequestSchema,
  type AuthUser as ProtoAuthUser,
  type SessionInfo as ProtoSessionInfo,
  RegisterRequestSchema,
  ResendVerificationEmailRequestSchema,
  RevokeSessionRequestSchema,
  SendMagicLinkRequestSchema,
  VerifyEmailRequestSchema,
  VerifyMagicLinkRequestSchema,
} from '@/gen/auth/v1/auth_pb';
import { clearCsrfToken, getCsrfToken, setCsrfToken, transport } from '@/lib/connect-transport';
import { captureHookError, setSentryUser } from '@/lib/sentry';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError, createClient } from '@connectrpc/connect';
import * as Sentry from '@sentry/react';
import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router';

/**
 * Reads the CSRF token from cookies.
 * The backend sets a non-HttpOnly cookie named 'csrf' that JavaScript can read.
 */
function getCsrfTokenFromCookie(): string | null {
  const cookies = document.cookie.split(';');
  for (const cookie of cookies) {
    const [name, value] = cookie.trim().split('=');
    if (name === 'csrf' || name === '__Host-csrf') {
      return value || null;
    }
  }
  return null;
}

/** i18n key for missing provider error */
const AUTH_PROVIDER_ERROR_KEY = 'contexts.auth.errors.providerMissing';

/**
 * AuthUser represents the authenticated user.
 */
export interface AuthUser {
  id: string;
  email: string;
  displayName?: string;
  role: string;
  emailVerified: boolean;
  createdAt: Date;
}

/**
 * SessionInfo represents the current session.
 */
export interface SessionInfo {
  sessionId: string;
  expiresAt: Date;
  deviceFingerprint?: string;
  ipAddress?: string;
  userAgent?: string;
  createdAt: Date;
  lastActivityAt: Date;
  isCurrent: boolean;
}

/**
 * AuthError represents an authentication error.
 */
export interface AuthError {
  code: string;
  message: string;
}

interface AuthContextType {
  user: AuthUser | null;
  session: SessionInfo | null;
  loading: boolean;
  signUp: (
    email: string,
    password: string,
    displayName?: string
  ) => Promise<{ error: AuthError | null }>;
  signIn: (email: string, password: string) => Promise<{ error: AuthError | null }>;
  signOut: (logoutAllDevices?: boolean) => Promise<void>;
  resetPasswordForEmail: (email: string) => Promise<{ error: AuthError | null }>;
  updatePassword: (
    currentPassword: string,
    newPassword: string
  ) => Promise<{ error: AuthError | null }>;
  verifyEmail: (token: string) => Promise<{ error: AuthError | null; user: AuthUser | null }>;
  resendVerificationEmail: () => Promise<{ error: AuthError | null }>;
  listSessions: () => Promise<{ sessions: SessionInfo[]; currentSessionId: string }>;
  revokeSession: (sessionId: string) => Promise<{ error: AuthError | null }>;
  sendMagicLink: (email: string) => Promise<{ error: AuthError | null }>;
  verifyMagicLink: (token: string) => Promise<{ error: AuthError | null }>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

/** ConnectRPC auth client */
const authClient = createClient(AuthService, transport);

/**
 * Converts protobuf Timestamp to Date.
 * Handles both protobuf-es format ({ seconds, nanos }) and plain Date objects.
 */
function timestampToDate(timestamp: unknown): Date {
  if (!timestamp) {
    return new Date();
  }
  // If it's already a Date
  if (timestamp instanceof Date) {
    return timestamp;
  }
  // If it has toDate method (some protobuf implementations)
  if (typeof (timestamp as { toDate?: () => Date }).toDate === 'function') {
    return (timestamp as { toDate: () => Date }).toDate();
  }
  // protobuf-es format: { seconds: bigint, nanos: number }
  const ts = timestamp as { seconds?: bigint | number | string; nanos?: number };
  if (ts.seconds !== undefined) {
    const seconds = typeof ts.seconds === 'bigint' ? Number(ts.seconds) : Number(ts.seconds);
    const nanos = ts.nanos || 0;
    return new Date(seconds * 1000 + nanos / 1000000);
  }
  // Fallback: try to parse as string or return current date
  if (typeof timestamp === 'string') {
    return new Date(timestamp);
  }
  return new Date();
}

/**
 * Converts proto AuthUser to local AuthUser.
 */
function protoToAuthUser(protoUser: ProtoAuthUser): AuthUser {
  return {
    id: protoUser.id,
    email: protoUser.email,
    displayName: protoUser.displayName,
    role: protoUser.role,
    emailVerified: protoUser.emailVerified,
    createdAt: timestampToDate(protoUser.createdAt),
  };
}

/**
 * Converts proto SessionInfo to local SessionInfo.
 */
function protoToSessionInfo(protoSession: ProtoSessionInfo): SessionInfo {
  return {
    sessionId: protoSession.sessionId,
    expiresAt: timestampToDate(protoSession.expiresAt),
    deviceFingerprint: protoSession.deviceFingerprint || undefined,
    ipAddress: protoSession.ipAddress || undefined,
    userAgent: protoSession.userAgent || undefined,
    createdAt: timestampToDate(protoSession.createdAt),
    lastActivityAt: timestampToDate(protoSession.lastActivityAt),
    isCurrent: protoSession.isCurrent,
  };
}

/**
 * Converts ConnectError to AuthError.
 */
function connectErrorToAuthError(error: unknown): AuthError {
  if (error instanceof ConnectError) {
    return {
      code: Code[error.code],
      message: error.message,
    };
  }
  if (error instanceof Error) {
    return {
      code: 'unknown',
      message: error.message,
    };
  }
  return {
    code: 'unknown',
    message: 'An unknown error occurred',
  };
}

/**
 * Gets browser fingerprint for device tracking.
 */
function getDeviceFingerprint(): string {
  const components = [
    navigator.userAgent,
    navigator.language,
    new Date().getTimezoneOffset().toString(),
    screen.width.toString(),
    screen.height.toString(),
    screen.colorDepth.toString(),
  ];

  const hash = components.join('|');
  let hashCode = 0;
  for (let i = 0; i < hash.length; i++) {
    const char = hash.charCodeAt(i);
    hashCode = (hashCode << 5) - hashCode + char;
    hashCode = hashCode & hashCode;
  }
  return Math.abs(hashCode).toString(16);
}

/**
 * Authentication provider using ConnectRPC.
 * Uses HTTP-only cookies for session management.
 */
export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [session, setSession] = useState<SessionInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();
  const initializedRef = useRef(false);

  /**
   * Checks current session on mount.
   * Only clears user state on explicit authentication errors (Unauthenticated).
   * Network errors and other transient failures preserve existing state.
   */
  useEffect(() => {
    // Fetches the current user and hydrates auth state.
    // Returns true when a valid user/session was restored, false otherwise.
    const hydrateFromCurrentUser = async (): Promise<boolean> => {
      const response = await authClient.getCurrentUser(create(GetCurrentUserRequestSchema, {}));

      if (!response.user || !response.session) {
        return false;
      }

      const restoredUser = protoToAuthUser(response.user);
      setUser(restoredUser);
      setSession(protoToSessionInfo(response.session));
      setSentryUser({
        id: restoredUser.id,
        email: restoredUser.email,
        role: restoredUser.role,
      });
      // Restore CSRF token from cookie if not in memory
      // This handles page reloads where the in-memory token is lost
      if (!getCsrfToken()) {
        const cookieToken = getCsrfTokenFromCookie();
        if (cookieToken) {
          setCsrfToken(cookieToken);
        }
      }
      return true;
    };

    const checkSession = async () => {
      try {
        if (await hydrateFromCurrentUser()) {
          return;
        }
        // Successful response but no user/session → genuinely logged out.
        setUser(null);
        setSession(null);
        clearCsrfToken();
      } catch (error) {
        // Only clear state on explicit authentication errors.
        // For network errors or transient failures, preserve existing state.
        if (error instanceof ConnectError && error.code === Code.Unauthenticated) {
          // A full page reload (e.g. forced by a deploy shipping a new bundle)
          // drops the in-memory access token, and GetCurrentUser is excluded
          // from the transport's auto-refresh interceptor. So recover here:
          // use the httpOnly refresh cookie to mint a fresh access token, then
          // retry once before treating the user as logged out. Without this a
          // still-valid session is thrown away on every reload.
          try {
            const refreshResponse = await authClient.refreshSession({});
            if (refreshResponse.csrfToken) {
              setCsrfToken(refreshResponse.csrfToken);
            }
            if (await hydrateFromCurrentUser()) {
              return;
            }
          } catch {
            // Refresh failed (no/expired session) → fall through to clearing state.
          }
          setUser(null);
          setSession(null);
          setSentryUser(null);
          clearCsrfToken();
        } else {
          // FSQ17: Capture non-auth errors as warning for diagnostics
          Sentry.captureException(error, {
            level: 'warning',
            tags: { auth_operation: 'checkSession' },
          });
        }
      } finally {
        initializedRef.current = true;
        setLoading(false);
      }
    };

    checkSession();
  }, []);

  /**
   * Registers a new user with email and password.
   */
  const signUp = useCallback(async (email: string, password: string, displayName?: string) => {
    try {
      await authClient.register(
        create(RegisterRequestSchema, {
          email,
          password,
          displayName,
        })
      );
      return { error: null };
    } catch (error) {
      return { error: connectErrorToAuthError(error) };
    }
  }, []);

  /**
   * Authenticates user with email and password.
   */
  const signIn = useCallback(async (email: string, password: string) => {
    try {
      const response = await authClient.login(
        create(LoginRequestSchema, {
          email,
          password,
          deviceFingerprint: getDeviceFingerprint(),
        })
      );

      if (response.user && response.session) {
        const loggedInUser = protoToAuthUser(response.user);
        setUser(loggedInUser);
        setSession(protoToSessionInfo(response.session));
        setCsrfToken(response.csrfToken);
        setSentryUser({ id: loggedInUser.id, email: loggedInUser.email, role: loggedInUser.role });
        // Navigation is handled by the calling component (use-auth-page)
        // via useEffect that monitors user state
        return { error: null };
      }

      return { error: { code: 'unknown', message: 'No user data returned' } };
    } catch (error) {
      return { error: connectErrorToAuthError(error) };
    }
  }, []);

  /**
   * Signs out the user.
   */
  const signOut = useCallback(
    async (logoutAllDevices = false) => {
      try {
        await authClient.logout(
          create(LogoutRequestSchema, {
            logoutAllDevices: Boolean(logoutAllDevices),
          })
        );
      } catch (error) {
        // FSQ15: Capture logout errors as warning instead of silencing
        Sentry.captureException(error, {
          level: 'warning',
          tags: { auth_operation: 'logout' },
        });
      }

      setSentryUser(null);
      setUser(null);
      setSession(null);
      clearCsrfToken();
      navigate(ROUTES.AUTH);
    },
    [navigate]
  );

  /**
   * Sends password reset email.
   */
  const resetPasswordForEmail = useCallback(async (email: string) => {
    try {
      await authClient.forgotPassword(create(ForgotPasswordRequestSchema, { email }));
      return { error: null };
    } catch (error) {
      return { error: connectErrorToAuthError(error) };
    }
  }, []);

  /**
   * Updates user password.
   */
  const updatePassword = useCallback(async (currentPassword: string, newPassword: string) => {
    try {
      await authClient.changePassword(
        create(ChangePasswordRequestSchema, {
          currentPassword,
          newPassword,
        })
      );
      return { error: null };
    } catch (error) {
      return { error: connectErrorToAuthError(error) };
    }
  }, []);

  /**
   * Verifies email with token.
   */
  const verifyEmail = useCallback(async (token: string) => {
    try {
      const response = await authClient.verifyEmail(create(VerifyEmailRequestSchema, { token }));
      const verifiedUser = response.user ? protoToAuthUser(response.user) : null;
      if (verifiedUser) {
        setUser(verifiedUser);
      }
      return { error: null, user: verifiedUser };
    } catch (error) {
      return { error: connectErrorToAuthError(error), user: null };
    }
  }, []);

  /**
   * Resends verification email.
   */
  const resendVerificationEmail = useCallback(async () => {
    try {
      await authClient.resendVerificationEmail(create(ResendVerificationEmailRequestSchema, {}));
      return { error: null };
    } catch (error) {
      return { error: connectErrorToAuthError(error) };
    }
  }, []);

  /**
   * Lists all sessions for current user.
   */
  const listSessions = useCallback(async () => {
    try {
      const response = await authClient.listSessions(create(ListSessionsRequestSchema, {}));
      return {
        sessions: response.sessions.map(protoToSessionInfo),
        currentSessionId: response.currentSessionId,
      };
    } catch (error) {
      // FSQ16: Capture server errors for diagnostics
      captureHookError(error, 'AuthContext', 'listSessions');
      return { sessions: [], currentSessionId: '' };
    }
  }, []);

  /**
   * Revokes a specific session.
   */
  const revokeSession = useCallback(async (sessionId: string) => {
    try {
      await authClient.revokeSession(create(RevokeSessionRequestSchema, { sessionId }));
      return { error: null };
    } catch (error) {
      return { error: connectErrorToAuthError(error) };
    }
  }, []);

  /**
   * Sends a magic link for passwordless authentication.
   */
  const sendMagicLink = useCallback(async (email: string) => {
    try {
      await authClient.sendMagicLink(create(SendMagicLinkRequestSchema, { email }));
      return { error: null };
    } catch (error) {
      return { error: connectErrorToAuthError(error) };
    }
  }, []);

  /**
   * Verifies a magic link token and creates a session.
   */
  const verifyMagicLink = useCallback(
    async (token: string) => {
      try {
        const response = await authClient.verifyMagicLink(
          create(VerifyMagicLinkRequestSchema, {
            token,
            deviceFingerprint: getDeviceFingerprint(),
          })
        );

        if (response.user && response.session) {
          const magicUser = protoToAuthUser(response.user);
          setUser(magicUser);
          setSession(protoToSessionInfo(response.session));
          setCsrfToken(response.csrfToken);
          setSentryUser({ id: magicUser.id, email: magicUser.email, role: magicUser.role });
          navigate(ROUTES.DASHBOARD);
          return { error: null };
        }

        return { error: { code: 'unknown', message: 'No user data returned' } };
      } catch (error) {
        return { error: connectErrorToAuthError(error) };
      }
    },
    [navigate]
  );

  return (
    <AuthContext.Provider
      value={{
        user,
        session,
        loading,
        signUp,
        signIn,
        signOut,
        resetPasswordForEmail,
        updatePassword,
        verifyEmail,
        resendVerificationEmail,
        listSessions,
        revokeSession,
        sendMagicLink,
        verifyMagicLink,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

/**
 * Hook to access auth context.
 */
export function useAuth() {
  const context = useContext(AuthContext);

  if (context === undefined) {
    throw new Error(AUTH_PROVIDER_ERROR_KEY);
  }

  return context;
}
