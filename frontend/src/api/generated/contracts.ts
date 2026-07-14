// Generated from docs/openapi.yaml. Do not edit.
export interface components {
  schemas: {
    "AuthCredentials": {
      "password": string;
      "username": string;
    };
    "AuthStatus": {
      "authenticated": boolean;
      "initialized": boolean;
      "user"?: components["schemas"]["SessionInfo"];
    };
    "AuthStatusEnvelope": {
      "data": components["schemas"]["AuthStatus"];
      "ok": true;
    };
    "DevelopmentKey": {
      "created_at": string;
      "id": string;
      "last_used_at"?: string;
      "name": string;
      "prefix": string;
      "revoked_at"?: string;
      "token"?: string;
    };
    "DevelopmentKeyEnvelope": {
      "data": components["schemas"]["DevelopmentKey"];
      "ok": true;
    };
    "DevelopmentKeyInput": {
      "name": string;
    };
    "DevelopmentKeyListEnvelope": {
      "data": Array<components["schemas"]["DevelopmentKey"]>;
      "ok": true;
    };
    "ErrorEnvelope": {
      "error": {
        "code": string;
        "message": string;
      };
      "ok": false;
    };
    "ImportCandidate": {
      "action": "new" | "update" | "unknown";
      "existing_mod_id"?: string;
      "file_name"?: string;
      "file_size"?: number;
      "id": string;
      "name"?: string;
      "package_name"?: string;
      "ready": boolean;
      "source_type": "workshop" | "github_asset" | "https_zip" | "local_zip";
      "version"?: string;
      "warnings"?: Array<string>;
    };
    "ImportInspection": {
      "candidates": Array<components["schemas"]["ImportCandidate"]>;
      "expires_at": string;
      "id": string;
      "selected_candidate_id"?: string;
      "source": string;
      "source_type": "workshop" | "github_release" | "https_zip" | "local_zip";
    };
    "ImportInspectionEnvelope": {
      "data": components["schemas"]["ImportInspection"];
      "ok": true;
    };
    "Job": {
      "created_at": string;
      "error"?: string;
      "error_code"?: string;
      "id": string;
      "message": string;
      "progress": number;
      "status": "queued" | "waiting" | "running" | "completed" | "failed";
      "type": string;
      "updated_at": string;
    };
    "JobEnvelope": {
      "data": components["schemas"]["Job"];
      "ok": true;
    };
    "JsonObject": Record<string, unknown>;
    "ModImportInspectRequest": {
      "source": string;
    };
    "ModImportRequest": {
      "candidate_id"?: string;
      "inspection_id": string;
    };
    "ModImportSelectRequest": {
      "candidate_id": string;
    };
    "ModImportUploadRequest": {
      "file": string;
    };
    "PalDefenderBroadcastRequest": {
      "alert"?: boolean;
      "message": string;
    };
    "PalDefenderGMInventory": {
      "Inventory": components["schemas"]["PalDefenderInventory"];
      "Meta": {
        "Player": string;
        "PlayerUID": string;
      };
    };
    "PalDefenderGMPlayer": {
      "GuildName": string;
      "GuildUUID": string;
      "IP": string;
      "MapLocation": components["schemas"]["PalDefenderLocation"];
      "Name": string;
      "PlayerUID": string;
      "Status": string;
      "UserId": string;
      "WorldLocation": components["schemas"]["PalDefenderLocation"];
    };
    "PalDefenderGMPlayers": {
      "Meta": {
        "OnlineCount": number;
        "PlayerCount": number;
      };
      "Players": Array<components["schemas"]["PalDefenderGMPlayer"]>;
    };
    "PalDefenderGMStatus": {
      "available": boolean;
      "configured": boolean;
      "error"?: string;
      "version"?: components["schemas"]["PalDefenderRESTVersion"];
    };
    "PalDefenderGiveItemsRequest": {
      "Items": Array<components["schemas"]["PalDefenderItemGrant"]>;
    };
    "PalDefenderInventory": {
      "Armor": components["schemas"]["PalDefenderInventoryContainer"];
      "DropSlot": components["schemas"]["PalDefenderInventoryContainer"];
      "Food": components["schemas"]["PalDefenderInventoryContainer"];
      "Items": components["schemas"]["PalDefenderInventoryContainer"];
      "KeyItems": components["schemas"]["PalDefenderInventoryContainer"];
      "Weapons": components["schemas"]["PalDefenderInventoryContainer"];
    };
    "PalDefenderInventoryContainer": {
      "Available": boolean;
      "ContainerID": string;
      "FreeSlots": number;
      "MaxSlots": number;
      "Slots": Record<string, components["schemas"]["PalDefenderInventorySlot"]>;
      "UsedSlots": number;
    };
    "PalDefenderInventorySlot": {
      "Count": number;
      "ItemID": string;
    };
    "PalDefenderItemCatalog": {
      "items": Array<components["schemas"]["PalDefenderItemCatalogEntry"]>;
      "returned": number;
    };
    "PalDefenderItemCatalogEntry": {
      "icon"?: string;
      "id": string;
      "name": string;
    };
    "PalDefenderItemGrant": {
      "Count": number;
      "ItemID": string;
    };
    "PalDefenderLocation": {
      "x": number;
      "y": number;
      "z": number;
    };
    "PalDefenderMessageRequest": {
      "Message": string;
      "SendType"?: "PlayerChat" | "PlayerGlobalChat" | "PlayerGuildChat" | "PlayerLogNormal" | "PlayerLogImportant" | "PlayerLogVeryImportant";
    };
    "PalDefenderPunishmentRequest": {
      "IP"?: boolean;
      "Reason"?: string;
    };
    "PalDefenderRESTVersion": {
      "Beta": boolean;
      "Build": number;
      "Major": number;
      "Minor": number;
      "Patch": number;
      "Version": string;
      "VersionLong": string;
    };
    "Schedule": components["schemas"]["ScheduleInput"] & {
      "created_at": string;
      "enabled": boolean;
      "id": string;
      "last_run_at"?: string;
      "next_run_at"?: string;
      "timezone": string;
      "updated_at": string;
    };
    "ScheduleInput": {
      "enabled"?: boolean;
      "interval_minutes"?: number;
      "message"?: string;
      "time_of_day"?: string;
      "timezone"?: string;
      "type": "save" | "backup" | "safe_restart" | "update" | "version_check";
      "waittime"?: number;
    };
    "SessionEnvelope": {
      "data": components["schemas"]["SessionInfo"];
      "ok": true;
    };
    "SessionInfo": {
      "name": string;
      "permissions": Array<string>;
      "role": "admin" | "operator" | "viewer";
    };
    "SuccessEnvelope": {
      "data": unknown;
      "ok": true;
    };
  };
}
