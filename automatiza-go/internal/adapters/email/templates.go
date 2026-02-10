package email

import (
	"bytes"
	"html/template"
)

const passwordResetTemplate = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Redefinir Senha - Automatiza</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); padding: 30px; border-radius: 10px 10px 0 0; text-align: center; }
        .header h1 { color: white; margin: 0; font-size: 24px; }
        .content { background: #f9fafb; padding: 30px; border: 1px solid #e5e7eb; }
        .button { display: inline-block; background: #667eea; color: white; padding: 14px 28px; text-decoration: none; border-radius: 6px; font-weight: 600; margin: 20px 0; }
        .button:hover { background: #5a67d8; }
        .warning { background: #fef3c7; border-left: 4px solid #f59e0b; padding: 12px; margin: 20px 0; font-size: 14px; }
        .footer { background: #f3f4f6; padding: 20px; border-radius: 0 0 10px 10px; font-size: 12px; color: #6b7280; text-align: center; }
        .details { background: #fff; padding: 15px; border-radius: 6px; margin: 15px 0; font-size: 13px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Automatiza</h1>
    </div>
    <div class="content">
        <h2>Redefinir sua senha</h2>
        <p>Ola {{.UserName}},</p>
        <p>Recebemos uma solicitacao para redefinir a senha da sua conta. Clique no botao abaixo para criar uma nova senha:</p>

        <p style="text-align: center;">
            <a href="{{.ResetURL}}" class="button">Redefinir Senha</a>
        </p>

        <div class="warning">
            <strong>Este link expira em {{.ExpiresIn}}.</strong><br>
            Se voce nao solicitou esta alteracao, ignore este email. Sua senha permanecera a mesma.
        </div>

        <div class="details">
            <strong>Detalhes da solicitacao:</strong><br>
            IP: {{.IPAddress}}<br>
            {{if .UserAgent}}Navegador: {{.UserAgent}}{{end}}
        </div>

        <p>Se o botao nao funcionar, copie e cole este link no seu navegador:</p>
        <p style="word-break: break-all; font-size: 12px; color: #6b7280;">{{.ResetURL}}</p>
    </div>
    <div class="footer">
        <p>&copy; 2026 Automatiza. Todos os direitos reservados.</p>
        <p>Este email foi enviado automaticamente. Por favor, nao responda.</p>
    </div>
</body>
</html>`

const passwordChangedTemplate = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Senha Alterada - Automatiza</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); padding: 30px; border-radius: 10px 10px 0 0; text-align: center; }
        .header h1 { color: white; margin: 0; font-size: 24px; }
        .content { background: #f9fafb; padding: 30px; border: 1px solid #e5e7eb; }
        .success { background: #d1fae5; border-left: 4px solid #10b981; padding: 12px; margin: 20px 0; }
        .warning { background: #fef3c7; border-left: 4px solid #f59e0b; padding: 12px; margin: 20px 0; font-size: 14px; }
        .footer { background: #f3f4f6; padding: 20px; border-radius: 0 0 10px 10px; font-size: 12px; color: #6b7280; text-align: center; }
        .details { background: #fff; padding: 15px; border-radius: 6px; margin: 15px 0; font-size: 13px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Automatiza</h1>
    </div>
    <div class="content">
        <h2>Senha alterada com sucesso</h2>
        <p>Ola {{.UserName}},</p>

        <div class="success">
            <strong>Sua senha foi alterada com sucesso!</strong>
        </div>

        <div class="details">
            <strong>Detalhes da alteracao:</strong><br>
            Data/Hora: {{.Timestamp}}<br>
            IP: {{.IPAddress}}
        </div>

        <div class="warning">
            <strong>Nao foi voce?</strong><br>
            Se voce nao realizou esta alteracao, entre em contato conosco imediatamente e altere sua senha.
        </div>
    </div>
    <div class="footer">
        <p>&copy; 2026 Automatiza. Todos os direitos reservados.</p>
        <p>Este email foi enviado automaticamente. Por favor, nao responda.</p>
    </div>
</body>
</html>`

const welcomeTemplate = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Bem-vindo ao Automatiza</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); padding: 30px; border-radius: 10px 10px 0 0; text-align: center; }
        .header h1 { color: white; margin: 0; font-size: 24px; }
        .content { background: #f9fafb; padding: 30px; border: 1px solid #e5e7eb; }
        .password-box { background: #fff; border: 2px dashed #667eea; padding: 20px; text-align: center; margin: 20px 0; border-radius: 8px; }
        .password { font-family: monospace; font-size: 24px; font-weight: bold; color: #667eea; letter-spacing: 2px; }
        .button { display: inline-block; background: #667eea; color: white; padding: 14px 28px; text-decoration: none; border-radius: 6px; font-weight: 600; margin: 20px 0; }
        .button:hover { background: #5a67d8; }
        .warning { background: #fef3c7; border-left: 4px solid #f59e0b; padding: 12px; margin: 20px 0; font-size: 14px; }
        .footer { background: #f3f4f6; padding: 20px; border-radius: 0 0 10px 10px; font-size: 12px; color: #6b7280; text-align: center; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Bem-vindo ao Automatiza!</h1>
    </div>
    <div class="content">
        <h2>Sua conta foi criada</h2>
        <p>Ola {{.UserName}},</p>
        <p>Sua conta no Automatiza foi criada com sucesso! Abaixo esta sua senha temporaria:</p>

        <div class="password-box">
            <p style="margin: 0 0 10px 0; font-size: 14px; color: #6b7280;">Sua senha temporaria:</p>
            <span class="password">{{.Password}}</span>
        </div>

        <div class="warning">
            <strong>Importante:</strong> Por favor, altere sua senha apos o primeiro login por questoes de seguranca.
        </div>

        <p>Agora voce pode acessar a plataforma e comecar a automatizar suas ofertas de afiliados no WhatsApp!</p>

        <p style="text-align: center;">
            <a href="{{.LoginURL}}" class="button">Acessar minha conta</a>
        </p>
    </div>
    <div class="footer">
        <p>&copy; 2026 Automatiza. Todos os direitos reservados.</p>
    </div>
</body>
</html>`

// Templates holds parsed HTML templates
type Templates struct {
	PasswordReset   *template.Template
	PasswordChanged *template.Template
	Welcome         *template.Template
}

// NewTemplates creates and parses all email templates
func NewTemplates() (*Templates, error) {
	passwordReset, err := template.New("password_reset").Parse(passwordResetTemplate)
	if err != nil {
		return nil, err
	}

	passwordChanged, err := template.New("password_changed").Parse(passwordChangedTemplate)
	if err != nil {
		return nil, err
	}

	welcome, err := template.New("welcome").Parse(welcomeTemplate)
	if err != nil {
		return nil, err
	}

	return &Templates{
		PasswordReset:   passwordReset,
		PasswordChanged: passwordChanged,
		Welcome:         welcome,
	}, nil
}

// RenderPasswordReset renders the password reset email template
func (t *Templates) RenderPasswordReset(input PasswordResetEmailInput) (string, error) {
	var buf bytes.Buffer
	if err := t.PasswordReset.Execute(&buf, input); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderPasswordChanged renders the password changed notification template
func (t *Templates) RenderPasswordChanged(input PasswordChangedEmailInput) (string, error) {
	var buf bytes.Buffer
	if err := t.PasswordChanged.Execute(&buf, input); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderWelcome renders the welcome email template
func (t *Templates) RenderWelcome(input WelcomeEmailInput) (string, error) {
	var buf bytes.Buffer
	if err := t.Welcome.Execute(&buf, input); err != nil {
		return "", err
	}
	return buf.String(), nil
}
