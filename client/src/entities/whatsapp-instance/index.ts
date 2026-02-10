export {
  extractWahaState,
  extractQrCode,
  hasRealInstanceName,
  INSTANCE_PLACEHOLDER_VALUES,
  isConnectedState,
  isConnectingState,
  isDisconnectedState,
} from './lib/waha-helpers';
export type {
  WahaCreateResponse,
  WahaGroup,
  WahaGroupsResponse,
  WahaStatusResponse,
  InstanceRow,
  InstanceStatus,
} from './model/types';
