// Generated from docs/openapi.yaml. Do not edit.
export interface components {
  schemas: {
    "AITranslationConfig": {
      "api_key_present": boolean;
      "base_url": string;
      "configured": boolean;
      "custom_header_names": Array<string>;
      "model": string;
      "proxy_configured": boolean;
      "proxy_url": string;
      "timeout_seconds": number;
    };
    "AITranslationConfigEnvelope": {
      "data": components["schemas"]["AITranslationConfig"];
      "ok": true;
    };
    "AITranslationConfigUpdate": {
      "api_key"?: string;
      "base_url"?: string;
      "clear_api_key"?: boolean;
      "clear_custom_headers"?: boolean;
      "clear_proxy"?: boolean;
      "custom_headers"?: Record<string, string>;
      "model"?: string;
      "proxy_url"?: string;
      "timeout_seconds"?: number;
    };
    "AITranslationTestEnvelope": {
      "data": components["schemas"]["AITranslationTestResult"];
      "ok": true;
    };
    "AITranslationTestResult": {
      "base_url": string;
      "custom_header_names": Array<string>;
      "message": string;
      "model": string;
      "ok": boolean;
      "proxy_configured": boolean;
      "timeout_seconds": number;
    };
    "AstrBotCommunityServerRequest": {
      "country"?: string;
      "limit"?: number;
      "query"?: string;
    };
    "AstrBotControlRequest": {
      "action": "start" | "safe_stop" | "safe_restart" | "force_stop";
      "actor_qq_id": string;
      "group_id"?: string;
      "message"?: string;
      "waittime"?: number;
    };
    "AstrBotOnlinePlayer": {
      "level"?: number;
      "name": string;
    };
    "AstrBotServerStatus": {
      "info"?: {
        "server_name"?: string;
        "version"?: string;
      };
      "online_count": number;
      "online_players": Array<components["schemas"]["AstrBotOnlinePlayer"]>;
      "players_available"?: boolean;
      "server": {
        "container": {
          "exists": boolean;
          "status": string;
        };
        "pending_restart": boolean;
        "runtime_mode": string;
        "setup_step": string;
      };
    };
    "AstrBotServerStatusEnvelope": {
      "data": components["schemas"]["AstrBotServerStatus"];
      "ok": true;
    };
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
    "BreedingStatus": {
      "available": boolean;
      "checked_at"?: string;
      "configured": boolean;
      "database_version"?: string;
      "last_error"?: string;
      "latency_ms": number;
      "upstream_version"?: string;
    };
    "BreedingStatusEnvelope": {
      "data": components["schemas"]["BreedingStatus"];
      "ok": true;
    };
    "CommunityServer": {
      "address": string;
      "connect": string;
      "country": string;
      "description"?: string;
      "id": string;
      "max_players": number;
      "name": string;
      "password": boolean;
      "players": number;
      "port": number;
      "status": "online" | "offline";
      "updated_at"?: string;
      "version"?: string;
    };
    "CommunityServerResult": {
      "cache_age_seconds": number;
      "fetched_at": string;
      "page": number;
      "page_size": number;
      "servers": Array<components["schemas"]["CommunityServer"]>;
      "source": "battlemetrics";
      "source_total": number;
      "stale": boolean;
      "total": number;
    };
    "CommunityServerResultEnvelope": {
      "data": components["schemas"]["CommunityServerResult"];
      "ok": true;
    };
    "CommunityServerSourceStatus": {
      "base_url": string;
      "cache_available": boolean;
      "cache_error"?: string;
      "cache_fresh": boolean;
      "cache_writable": boolean;
      "cached_queries": number;
      "enabled": boolean;
      "last_attempt_at"?: string;
      "last_error"?: string;
      "last_success_at"?: string;
      "next_refresh_at"?: string;
      "proxy_configured": boolean;
      "rate_limit_per_minute": number;
      "reachable": boolean;
      "source": "battlemetrics";
    };
    "CommunityServerSourceStatusEnvelope": {
      "data": components["schemas"]["CommunityServerSourceStatus"];
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
    "LocalModActionCapability": {
      "action": "import" | "repair" | "ignore" | "unignore" | "delete";
      "available": boolean;
      "confirmation_required": boolean;
      "reason"?: string;
    };
    "LocalModActionEnvelope": {
      "data": components["schemas"]["LocalModActionResult"];
      "ok": true;
    };
    "LocalModActionRequest": {
      "action": "import" | "repair" | "ignore" | "unignore" | "delete";
      "confirm"?: boolean;
      "revision": string;
    };
    "LocalModActionResult": {
      "action": "import" | "repair" | "ignore" | "unignore" | "delete";
      "finding_id": string;
      "message": string;
      "mod"?: components["schemas"]["ModRecord"];
      "scan": components["schemas"]["LocalScanResult"];
    };
    "LocalModFinding": {
      "actions": Array<components["schemas"]["LocalModActionCapability"]>;
      "classifications": Array<"managed" | "manual" | "present" | "missing_files" | "unknown" | "disabled" | "duplicate" | "incomplete">;
      "confidence": "high" | "medium" | "low";
      "database_mods"?: Array<components["schemas"]["ModRecord"]>;
      "duplicate": boolean;
      "enabled": boolean;
      "id": string;
      "ignored": boolean;
      "issues"?: Array<string>;
      "name": string;
      "ownership": "managed" | "manual";
      "package_name"?: string;
      "paths": Array<string>;
      "revision": string;
      "source": "workshop" | "legacy_pak" | "ue4ss" | "database";
      "state": "present" | "missing_files" | "unknown" | "disabled" | "duplicate" | "incomplete";
      "version"?: string;
    };
    "LocalScanEnvelope": {
      "data": components["schemas"]["LocalScanResult"];
      "ok": true;
    };
    "LocalScanResult": {
      "findings": Array<components["schemas"]["LocalModFinding"]>;
      "scanned_at": string;
      "server_dir": string;
      "skipped_paths": Array<string>;
      "warnings": Array<string>;
    };
    "ModConfigBackup": {
      "created_at": string;
      "id": string;
      "revision": string;
      "size": number;
    };
    "ModConfigBackupListEnvelope": {
      "data": Array<components["schemas"]["ModConfigBackup"]>;
      "ok": true;
    };
    "ModConfigDocument": {
      "content": string;
      "fields"?: Array<components["schemas"]["ModConfigurationField"]>;
      "file": components["schemas"]["ModConfigFile"];
      "format": string;
    };
    "ModConfigDocumentEnvelope": {
      "data": components["schemas"]["ModConfigDocument"];
      "ok": true;
    };
    "ModConfigFile": {
      "executable": boolean;
      "extension": ".json" | ".ini" | ".cfg" | ".toml" | ".yaml" | ".yml" | ".txt" | ".lua";
      "id": string;
      "modified_at": string;
      "name": string;
      "path": string;
      "revision": string;
      "risk"?: string;
      "size": number;
    };
    "ModConfigFileListEnvelope": {
      "data": Array<components["schemas"]["ModConfigFile"]>;
      "ok": true;
    };
    "ModConfigRestoreRequest": {
      "revision": string;
    };
    "ModConfigWriteRequest": {
      "confirm_executable"?: boolean;
      "content": string;
      "revision": string;
    };
    "ModConfigurationAdapter": {
      "available": boolean;
      "description": string;
      "files": Array<components["schemas"]["ModConfigFile"]>;
      "id": string;
      "name": string;
      "reload_behavior": "online_reload" | "restart_required";
      "workshop_id"?: string;
    };
    "ModConfigurationAdapterListEnvelope": {
      "data": Array<components["schemas"]["ModConfigurationAdapter"]>;
      "ok": true;
    };
    "ModConfigurationField": {
      "label": string;
      "max"?: number;
      "min"?: number;
      "path": string;
      "type": "boolean" | "integer" | "number" | "string";
      "value": unknown;
    };
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
    "ModRecord": {
      "created_at": string;
      "enabled": boolean;
      "file_size"?: number;
      "id": string;
      "last_checked_at"?: string;
      "name": string;
      "package_name": string;
      "path": string;
      "preview_url"?: string;
      "source": string;
      "steam_url"?: string;
      "subscriptions"?: number;
      "summary"?: string;
      "tags"?: Array<string>;
      "time_updated"?: number;
      "updated_at": string;
      "version"?: string;
      "workshop_id"?: string;
    };
    "PalDefenderAccessSettingsUpdate": {
      "admin_auto_login": boolean;
      "admin_ips": Array<string>;
      "use_admin_whitelist": boolean;
      "use_whitelist": boolean;
      "whitelist_message": string;
    };
    "PalDefenderBroadcastRequest": {
      "alert"?: boolean;
      "message": string;
    };
    "PalDefenderExportedPalTemplateInfo": {
      "modified_at": string;
      "name": string;
      "path": string;
      "player_id": string;
      "size": number;
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
      "installed": boolean;
      "load_verified": boolean;
      "rest_enabled": boolean;
      "state": "ready" | "not_installed" | "not_loaded" | "not_configured" | "rest_disabled" | "server_not_running" | "failed";
      "version"?: components["schemas"]["PalDefenderRESTVersion"];
    };
    "PalDefenderGiveItemsRequest": {
      "Items": Array<components["schemas"]["PalDefenderItemGrant"]>;
    };
    "PalDefenderGivePalTemplatesRequest": {
      "PalTemplates": Array<string>;
    };
    "PalDefenderGivePalsRequest": {
      "Pals": Array<components["schemas"]["PalDefenderPalGrant"]>;
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
    "PalDefenderPalGrant": {
      "Level": number;
      "PalID": string;
    };
    "PalDefenderPalTemplate": {
      "ActiveSkills"?: Array<string>;
      "CondensedPals"?: number;
      "CraftSpeed"?: number;
      "DisableWorkPreferences"?: Array<string>;
      "Exp"?: number;
      "ExtraWorkSuitabilities"?: Record<string, number>;
      "FriendshipPoints"?: number;
      "Gender"?: "Male" | "Female" | "None";
      "HP"?: number;
      "Hunger"?: number;
      "IVs"?: Record<string, number>;
      "ImportedCharacter"?: boolean;
      "LearntSkills"?: Array<string>;
      "Level"?: number;
      "MP"?: number;
      "MaxHunger"?: number;
      "Nickname"?: string;
      "PalID": string;
      "PalSouls"?: Record<string, number>;
      "PartnerSkillLevel"?: number;
      "Passives"?: Array<string>;
      "PhysicalHealth"?: string;
      "SAN"?: number;
      "SP"?: number;
      "Shield"?: number;
      "Shiny"?: boolean;
      "SkinId"?: string;
      "Support"?: number;
      "UniqueNPCID"?: string;
      "UnusedStatusPoints"?: number;
      "WorkerSick"?: string;
    };
    "PalDefenderProgressionGrantRequest": {
      "AncientTechnologyPoints"?: number;
      "EXP"?: number;
      "Relics"?: Record<string, number>;
      "TechnologyPoints"?: number;
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
    "PalDefenderTechnologyRequest": {
      "Technology": unknown;
    };
    "SafeLifecycleRequest": {
      "message"?: string;
      "waittime"?: number;
    };
    "SaveSourceImportRequest": {
      "file": string;
      "name"?: string;
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
    "SteamWorkshopAuthRequest": {
      "account_name"?: string;
    };
    "SteamWorkshopAuthStatus": {
      "account_name"?: string;
      "credentials_secure": boolean;
      "last_verified_at"?: string;
      "logged_in": boolean;
      "login_in_progress": boolean;
      "message"?: string;
      "steamcmd_installed": boolean;
      "supported": boolean;
      "verification_required": boolean;
    };
    "SteamWorkshopAuthStatusEnvelope": {
      "data": components["schemas"]["SteamWorkshopAuthStatus"];
      "ok": true;
    };
    "SuccessEnvelope": {
      "data": unknown;
      "ok": true;
    };
    "WebDAVConfig": {
      "base_url": string;
      "enabled": boolean;
      "password_configured": boolean;
      "remote_path": string;
      "upload_after_backup": boolean;
      "username": string;
    };
    "WebDAVConfigUpdate": {
      "base_url"?: string;
      "clear_password"?: boolean;
      "enabled"?: boolean;
      "password"?: string;
      "remote_path"?: string;
      "upload_after_backup"?: boolean;
      "username"?: string;
    };
  };
}
