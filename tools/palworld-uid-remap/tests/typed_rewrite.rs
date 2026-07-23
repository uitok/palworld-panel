use palworld_uid_remap::{rewrite_typed_tree, CandidateKind, FieldCategory, MappingSet};
use uesave::games::palworld::{
    PalDynamicId, PalGroupData, PalGroupVariant, PalGuildGroup, PalGuildMarker, PalGuildTail,
    PalGuildTailPostUpdate, PalGuildTailPreUpdate, PalGuildPlayerWithRole, PalGuildRolePermission,
    PalInstanceId, PalItemAndNum, PalItemId, PalMapConcreteModel, PalMapConcreteModelModule,
    PalMapConcreteModelModuleData, PalMapConcreteModelVariant, PalMapModel,
    PalMapObjectDeathDroppedCharacterModel, PalMapObjectHp, PalMapObjectItemBoothModel,
    PalMapObjectItemBoothTradeInfo, PalPlayerInfo, PalPlayerInfoDetails, PalStageInstanceId,
    PalStruct, PalTransform, Palworld,
};
use uesave::{
    ByteArray, Double, FGuid, MapEntry, Properties, Property, Quat, SaveGameArchiveType,
    StructValue, ValueVec, Vector,
};

type PalProperty = Property<SaveGameArchiveType<Palworld>>;

const A: &str = "00112233-4455-6677-8899-aabbccddeeff";
const B: &str = "11112233-4455-6677-8899-aabbccddeeff";
const STABLE: &str = "99112233-4455-6677-8899-aabbccddeeff";
const OTHER_INSTANCE: &str = "88112233-4455-6677-8899-aabbccddeeff";

fn uid(value: &str) -> FGuid {
    FGuid::parse_str(value).unwrap()
}

fn mapping() -> MappingSet {
    MappingSet::from_json(
        format!(r#"{{"source_uid":"{A}","target_uid":"{B}"}}"#).as_bytes(),
    )
    .unwrap()
}

fn guid(value: &str) -> PalProperty {
    Property::Struct(StructValue::Guid(uid(value)))
}

fn nested(entries: impl IntoIterator<Item = (&'static str, PalProperty)>) -> PalProperty {
    let mut properties = Properties::default();
    for (name, value) in entries {
        properties.insert(name, value);
    }
    Property::Struct(StructValue::Struct(properties))
}

#[test]
fn rewrites_player_save_character_index_and_pal_ownership_but_not_other_guids() {
    let mut save_data = Properties::default();
    save_data.insert("PlayerUId", guid(A));
    save_data.insert("IndividualId", nested([("PlayerUId", guid(A)), ("InstanceId", guid(STABLE))]));

    let mut character_key = Properties::default();
    character_key.insert("PlayerUId", guid(A));
    character_key.insert("InstanceId", guid(STABLE));

    let mut pal = Properties::default();
    pal.insert("OwnerPlayerUId", guid(A));
    pal.insert(
        "OldOwnerPlayerUIds",
        Property::Array(ValueVec::Struct(vec![StructValue::Guid(uid(A))])),
    );
    pal.insert("UnrelatedGuid", guid(A));

    let mut player_root = Properties::default();
    player_root.insert("SaveData", Property::Struct(StructValue::Struct(save_data)));
    let player_report = rewrite_typed_tree(
        &mut player_root,
        &mapping(),
        "Players/00112233445566778899AABBCCDDEEFF.sav",
    );

    let mut level_root = Properties::default();
    level_root.insert(
        "CharacterSaveParameterMap",
        Property::Map(vec![MapEntry {
            key: Property::Struct(StructValue::Struct(character_key)),
            value: Property::Struct(StructValue::Struct(pal)),
        }]),
    );

    let level_report = rewrite_typed_tree(&mut level_root, &mapping(), "Level.sav");
    let player_json = serde_json::to_value(&player_root).unwrap().to_string();
    let level_json = serde_json::to_value(&level_root).unwrap().to_string();
    assert_eq!(player_json.matches(B).count(), 2);
    assert_eq!(level_json.matches(B).count(), 3);
    assert_eq!(level_json.matches(A).count(), 1, "only UnrelatedGuid remains");
    assert!(player_json.contains(STABLE));
    assert!(level_json.contains(STABLE));
    assert_eq!(player_report.count(FieldCategory::PlayerSave), 2);
    assert_eq!(level_report.count(FieldCategory::CharacterIndex), 1);
    assert_eq!(level_report.count(FieldCategory::PalOwnership), 2);
}

fn guild(tail: PalGuildTail, handle_instance: &str) -> PalProperty {
    Property::Struct(StructValue::Game(PalStruct::GroupData(PalGroupData {
        group_id: uid(STABLE),
        group_name: "synthetic".into(),
        individual_character_handle_ids: vec![PalInstanceId {
            guid: uid(A),
            instance_id: uid(handle_instance),
        }],
        data: PalGroupVariant::Guild(PalGuildGroup {
            org_type: 0,
            leading_bytes: [0; 4],
            base_ids: vec![],
            unknown_1: 0,
            base_camp_level: 1,
            map_object_instance_ids_base_camp_points: vec![],
            guild_name: "synthetic".into(),
            last_guild_name_modifier_player_uid: uid(A),
            guild_markers: vec![PalGuildMarker {
                marker_id: uid(STABLE),
                icon_location: Vector {
                    x: Double(0.0),
                    y: Double(0.0),
                    z: Double(0.0),
                },
                icon_type: 0,
                owner_player_uid: uid(A),
            }],
            tail,
        }),
    })))
}

#[test]
fn rewrites_both_guild_tail_shapes_and_character_handles() {
    let details = PalPlayerInfoDetails {
        last_online_real_time: 7,
        player_name: "synthetic".into(),
    };
    let pre = PalGuildTail::PreUpdate(PalGuildTailPreUpdate {
        admin_player_uid: uid(A),
        players: vec![PalPlayerInfo {
            player_uid: uid(A),
            player_info: details.clone(),
        }],
        trailing_bytes: [0; 4],
    });
    let post = PalGuildTail::PostUpdate(PalGuildTailPostUpdate {
        guild_chest_allowed_roles: vec![1],
        unknown_i32: 0,
        admin_player_uid: uid(A),
        players: vec![PalGuildPlayerWithRole {
            player_uid: uid(A),
            player_info: details,
            role: 1,
        }],
        role_permissions: vec![PalGuildRolePermission {
            role: 1,
            permissions: vec![1],
        }],
        trailing_bytes: [0; 4],
    });
    let mut root = Properties::default();
    let mut character_key = Properties::default();
    character_key.insert("PlayerUId", guid(A));
    character_key.insert("InstanceId", guid(STABLE));
    root.insert(
        "CharacterSaveParameterMap",
        Property::Map(vec![MapEntry {
            key: Property::Struct(StructValue::Struct(character_key)),
            value: Property::Struct(StructValue::Struct(Properties::default())),
        }]),
    );
    root.insert("GuildPre", guild(pre, STABLE));
    root.insert("GuildPost", guild(post, OTHER_INSTANCE));

    let report = rewrite_typed_tree(&mut root, &mapping(), "Level.sav");
    let json = serde_json::to_value(&root).unwrap().to_string();
    assert_eq!(json.matches(A).count(), 1, "unbound guild handle is preserved");
    assert_eq!(report.count(FieldCategory::CharacterIndex), 1);
    assert_eq!(report.count(FieldCategory::Guild), 9);
}

#[test]
fn exact_path_allowlist_ignores_lookalike_player_uid_paths() {
    let mut fake_save = Properties::default();
    fake_save.insert("PlayerUId", guid(A));
    let mut fake_index = Properties::default();
    fake_index.insert("PlayerUId", guid(A));
    let mut root = Properties::default();
    root.insert(
        "NotSaveData",
        Property::Struct(StructValue::Struct(fake_save)),
    );
    root.insert(
        "CharacterSaveParameterMapBackup",
        Property::Struct(StructValue::Struct(fake_index)),
    );

    let report = rewrite_typed_tree(&mut root, &mapping(), "Level.sav");
    let json = serde_json::to_value(&root).unwrap().to_string();
    assert_eq!(json.matches(A).count(), 2);
    assert!(report.rewritten_fields.is_empty());
}

fn item() -> PalItemAndNum {
    PalItemAndNum {
        item_id: PalItemId {
            static_id: "synthetic".into(),
            dynamic_id: PalDynamicId {
                created_world_id: FGuid::nil(),
                local_id_in_created_world: FGuid::nil(),
            },
        },
        num: 1,
    }
}

#[test]
fn rewrites_build_map_lock_and_item_booth_fields() {
    let zero_vector = Vector {
        x: Double(0.0),
        y: Double(0.0),
        z: Double(0.0),
    };
    let map_model = PalMapModel {
        instance_id: uid(STABLE),
        concrete_model_instance_id: FGuid::nil(),
        base_camp_id_belong_to: FGuid::nil(),
        group_id_belong_to: FGuid::nil(),
        hp: PalMapObjectHp { current: 1, max: 1 },
        initial_transform_cache: PalTransform {
            rotation: Quat {
                x: Double(0.0),
                y: Double(0.0),
                z: Double(0.0),
                w: Double(1.0),
            },
            translation: zero_vector.clone(),
            scale: zero_vector,
        },
        repair_work_id: FGuid::nil(),
        owner_spawner_level_object_instance_id: FGuid::nil(),
        owner_instance_id: FGuid::nil(),
        build_player_uid: uid(A),
        interact_restrict_type: 0,
        deterioration_damage: 0.0,
        stage_instance_id_belong_to: PalStageInstanceId {
            id: FGuid::nil(),
            valid: 0,
        },
        unknown_bytes: vec![],
    };
    let booth = PalMapConcreteModel {
        instance_id: uid(STABLE),
        model_instance_id: FGuid::nil(),
        concrete_model_type: "PalMapObjectItemBoothModel".into(),
        model_data: PalMapConcreteModelVariant::ItemBooth(PalMapObjectItemBoothModel {
            leading_bytes: [0; 4],
            private_lock_player_uid: uid(A),
            trade_infos: vec![PalMapObjectItemBoothTradeInfo {
                product: item(),
                cost: item(),
                seller_player_uid: uid(A),
            }],
            trailing_bytes: [0; 20],
        }),
    };
    let lock = PalMapConcreteModelModule {
        module_type: "EPalMapObjectConcreteModelModuleType::PasswordLock".into(),
        data: PalMapConcreteModelModuleData::PasswordLock {
            lock_state: 1,
            password: "synthetic".into(),
            player_infos: vec![uesave::games::palworld::PalPlayerLockInfo {
                player_uid: uid(A),
                try_failed_count: 0,
                try_success_cache: 0,
            }],
            trailing_bytes: [0; 4],
        },
        custom_version_data: vec![],
    };
    let mut root = Properties::default();
    root.insert("MapModel", Property::Struct(StructValue::Game(PalStruct::MapModel(Box::new(map_model)))));
    root.insert("Booth", Property::Struct(StructValue::Game(PalStruct::MapConcreteModel(Box::new(booth)))));
    root.insert("Lock", Property::Struct(StructValue::Game(PalStruct::MapConcreteModelModule(lock))));

    let report = rewrite_typed_tree(&mut root, &mapping(), "Level.sav");
    let json = serde_json::to_value(&root).unwrap().to_string();
    assert_eq!(json.matches(A).count(), 0);
    assert_eq!(json.matches(STABLE).count(), 2);
    assert_eq!(report.count(FieldCategory::BuildOwnership), 1);
    assert_eq!(report.count(FieldCategory::Lock), 2);
    assert_eq!(report.count(FieldCategory::Trading), 1);
}

#[test]
fn rewrites_death_dropped_character_map_owner_only() {
    let death_drop = PalMapConcreteModel {
        instance_id: uid(STABLE),
        model_instance_id: FGuid::nil(),
        concrete_model_type: "PalMapObjectDeathDroppedCharacterModel".into(),
        model_data: PalMapConcreteModelVariant::DeathDroppedCharacter(
            PalMapObjectDeathDroppedCharacterModel {
                stored_parameter_id: uid(STABLE),
                owner_player_uid: uid(A),
                trailing_bytes: vec![],
            },
        ),
    };
    let mut root = Properties::default();
    root.insert(
        "DeathDrop",
        Property::Struct(StructValue::Game(PalStruct::MapConcreteModel(Box::new(
            death_drop,
        )))),
    );

    let report = rewrite_typed_tree(&mut root, &mapping(), "Level.sav");
    let json = serde_json::to_value(&root).unwrap().to_string();
    assert_eq!(json.matches(A).count(), 0);
    assert_eq!(json.matches(B).count(), 1);
    assert_eq!(json.matches(STABLE).count(), 2);
    assert_eq!(report.count(FieldCategory::MapOwnership), 1);
}

#[test]
fn reports_opaque_source_and_target_candidates_without_patching_bytes() {
    let mut bytes = vec![9, 9];
    bytes.extend(pal_guid_bytes(A));
    bytes.extend(pal_guid_bytes(B));
    let original = bytes.clone();
    let mut root = Properties::default();
    root.insert(
        "RawData",
        Property::Array(ValueVec::Byte(ByteArray::Byte(bytes))),
    );

    let report = rewrite_typed_tree(&mut root, &mapping(), "Level.sav");
    let Property::Array(ValueVec::Byte(ByteArray::Byte(after))) = &root["RawData"] else {
        panic!("fixture changed shape");
    };
    assert_eq!(after, &original);
    assert_eq!(report.opaque_candidates.len(), 2);
    assert_eq!(report.opaque_candidates[0].file, "Level.sav");
    assert_eq!(report.opaque_candidates[0].path, "RawData");
    assert_eq!(report.opaque_candidates[0].kind, CandidateKind::Source);
    assert_eq!(report.opaque_candidates[1].kind, CandidateKind::Target);
}

fn pal_guid_bytes(value: &str) -> Vec<u8> {
    value
        .replace('-', "")
        .as_bytes()
        .chunks_exact(8)
        .flat_map(|chunk| {
            let text = std::str::from_utf8(chunk).unwrap();
            u32::from_str_radix(text, 16).unwrap().to_le_bytes()
        })
        .collect()
}
