export type OperationType = "insert" | "delete" | "cursor";

export interface OperationMessage {
  type: OperationType;
  docId: string;
  position: number;
  value?: string;
  operationId: string;
  source: string;
  timestamp: number;
  cursorPos?: number;
  userColor?: string;
}

export interface InitMessage {
  type: "init";
  docId: string;
  text: string;
  presence: Record<string, PresenceInfo>;
}

export interface PresenceInfo {
  userID: string;
  userColor: string;
  cursorPos: number;
}

export type ServerMessage = InitMessage | OperationMessage;
