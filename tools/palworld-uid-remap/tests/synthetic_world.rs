use std::fs::{self, File};

use palworld_uid_remap::{
    remap_world, FieldCategory, MappingSet, RemapError, RemapOptions,
};
use tempfile::TempDir;
use uesave::games::palworld::Palworld;
use uesave::{
    ByteArray, FGuid, Header, MapEntry, PackageVersion, Properties, Property, PropertySchemas,
    PropertyTagDataPartial, PropertyTagPartial, Root, Save, SaveGameArchiveType, StructType,
    StructValue, ValueVec,
};

type PalProperties = Properties<SaveGameArchiveType<Palworld>>;
type PalProperty = Property<SaveGameArchiveType<Palworld>>;

const A: &str = "00112233-4455-6677-8899-aabbccddeeff";
const B: &str = "11112233-4455-6677-8899-aabbccddeeff";
const C: &str = "20112233-4455-6677-8899-aabbccddeeff";
const D: &str = "31112233-4455-6677-8899-aabbccddeeff";
const INSTANCE: &str = "99112233-4455-6677-8899-aabbccddeeff";
const INSTANCE_C: &str = "88112233-4455-6677-8899-aabbccddeeff";

#[test]
fn remaps_a_generated_world_and_preserves_unrelated_bytes() {
    let temp = TempDir::new().unwrap();
    let input = temp.path().join("world");
    let output = temp.path().join("remapped");
    fs::create_dir_all(input.join("Players")).unwrap();
    write_level(&input.join("Level.sav"), A, None);
    write_player(&input.join("Players").join(player_name(A)));
    fs::write(input.join("notes.bin"), b"synthetic-unrelated\0bytes").unwrap();

    let before_level = fs::read(input.join("Level.sav")).unwrap();
    let before_player = fs::read(input.join("Players").join(player_name(A))).unwrap();
    let mapping = MappingSet::from_json(
        format!(r#"{{"source_uid":"{A}","target_uid":"{B}"}}"#).as_bytes(),
    )
    .unwrap();
    let report = remap_world(&input, &output, &mapping, &RemapOptions::default()).unwrap();

    assert!(output.join("Level.sav").is_file());
    assert!(!output.join("Players").join(player_name(A)).exists());
    assert!(output.join("Players").join(player_name(B)).is_file());
    assert_eq!(
        fs::read(output.join("notes.bin")).unwrap(),
        b"synthetic-unrelated\0bytes"
    );
    assert_eq!(fs::read(input.join("Level.sav")).unwrap(), before_level);
    assert_eq!(
        fs::read(input.join("Players").join(player_name(A))).unwrap(),
        before_player
    );
    assert_eq!(report.rewritten_fields[&FieldCategory::PlayerSave], 2);
    assert_eq!(report.rewritten_fields[&FieldCategory::CharacterIndex], 1);
    assert!(report.opaque_candidates.is_empty());
}

#[test]
fn opaque_source_and_target_candidates_are_fatal() {
    let source_case = TempDir::new().unwrap();
    let source_input = source_case.path().join("world");
    let source_output = source_case.path().join("output");
    fs::create_dir_all(source_input.join("Players")).unwrap();
    write_level(&source_input.join("Level.sav"), A, Some(A));
    write_player(&source_input.join("Players").join(player_name(A)));
    let mapping = mapping();
    let error = remap_world(
        &source_input,
        &source_output,
        &mapping,
        &RemapOptions::default(),
    )
    .unwrap_err();
    assert!(matches!(error, RemapError::OpaqueSourceReference { .. }));
    assert!(!source_output.exists());

    let target_case = TempDir::new().unwrap();
    let target_input = target_case.path().join("world");
    let target_output = target_case.path().join("output");
    fs::create_dir_all(target_input.join("Players")).unwrap();
    write_level(&target_input.join("Level.sav"), A, Some(B));
    write_player(&target_input.join("Players").join(player_name(A)));
    let error = remap_world(
        &target_input,
        &target_output,
        &mapping,
        &RemapOptions::default(),
    )
    .unwrap_err();
    assert!(
        error.to_string().contains("opaque target UID reference"),
        "{error:?}"
    );
    assert!(!target_output.exists());
}

#[test]
fn top_level_gvas_extra_opaque_candidate_is_fatal() {
    let temp = TempDir::new().unwrap();
    let input = temp.path().join("world");
    let output = temp.path().join("output");
    fs::create_dir_all(input.join("Players")).unwrap();
    write_level_with_identities(
        &input.join("Level.sav"),
        &[(A, INSTANCE)],
        None,
        pal_guid_bytes(A),
    );
    write_player(&input.join("Players").join(player_name(A)));

    let error = remap_world(&input, &output, &mapping(), &RemapOptions::default()).unwrap_err();
    assert!(matches!(error, RemapError::OpaqueSourceReference { path, .. } if path == "<gvas-extra>"));
    assert!(!output.exists());
}

#[test]
fn rejects_typed_target_uid_references_already_present_in_input() {
    let temp = TempDir::new().unwrap();
    let input = temp.path().join("world");
    let output = temp.path().join("output");
    fs::create_dir_all(input.join("Players")).unwrap();
    write_level_with_players(&input.join("Level.sav"), &[A, B], None);
    write_player(&input.join("Players").join(player_name(A)));

    let error = remap_world(&input, &output, &mapping(), &RemapOptions::default()).unwrap_err();
    assert!(
        error.to_string().contains("target UID typed reference"),
        "{error:?}"
    );
    assert!(!output.exists());
}

#[test]
fn rejects_source_uid_absent_from_character_index() {
    let temp = TempDir::new().unwrap();
    let input = temp.path().join("world");
    let output = temp.path().join("output");
    fs::create_dir_all(input.join("Players")).unwrap();
    write_level(&input.join("Level.sav"), INSTANCE, None);
    write_player(&input.join("Players").join(player_name(A)));
    let error = remap_world(&input, &output, &mapping(), &RemapOptions::default()).unwrap_err();
    assert!(matches!(error, RemapError::Gate(message) if message.contains("absent from parsed player index")));
    assert!(!output.exists());
}

#[test]
fn remaps_multiple_players_and_dps_files_in_one_operation() {
    let temp = TempDir::new().unwrap();
    let input = temp.path().join("world");
    let output = temp.path().join("output");
    fs::create_dir_all(input.join("Players")).unwrap();
    write_level_with_identities(
        &input.join("Level.sav"),
        &[(A, INSTANCE), (C, INSTANCE_C)],
        None,
        vec![],
    );
    for (player_uid, instance_id) in [(A, INSTANCE), (C, INSTANCE_C)] {
        write_player_for(
            &input.join("Players").join(player_name(player_uid)),
            player_uid,
            instance_id,
        );
        write_player_for(
            &input.join("Players").join(dps_name(player_uid)),
            player_uid,
            instance_id,
        );
    }
    let mapping = MappingSet::from_json(
        format!(
            r#"[{{"source_uid":"{C}","target_uid":"{D}"}},{{"source_uid":"{A}","target_uid":"{B}"}}]"#
        )
        .as_bytes(),
    )
    .unwrap();

    let report = remap_world(&input, &output, &mapping, &RemapOptions::default()).unwrap();

    for source in [A, C] {
        assert!(!output.join("Players").join(player_name(source)).exists());
        assert!(!output.join("Players").join(dps_name(source)).exists());
    }
    for target in [B, D] {
        assert!(output.join("Players").join(player_name(target)).is_file());
        assert!(output.join("Players").join(dps_name(target)).is_file());
    }
    assert_eq!(report.rewritten_fields[&FieldCategory::PlayerSave], 8);
    assert_eq!(report.rewritten_fields[&FieldCategory::CharacterIndex], 2);
    assert_eq!(report.mappings[0], (A.to_owned(), B.to_owned()));
    assert_eq!(report.mappings[1], (C.to_owned(), D.to_owned()));
}

fn write_player(path: &std::path::Path) {
    write_player_for(path, A, INSTANCE);
}

fn write_player_for(path: &std::path::Path, player_uid: &str, instance_id: &str) {
    let mut individual = Properties::default();
    individual.insert("PlayerUId", guid(player_uid));
    individual.insert("InstanceId", guid(instance_id));
    let mut save_data = Properties::default();
    save_data.insert("PlayerUId", guid(player_uid));
    save_data.insert(
        "IndividualId",
        Property::Struct(StructValue::Struct(individual)),
    );
    let mut root = Properties::default();
    root.insert("SaveData", Property::Struct(StructValue::Struct(save_data)));

    let mut schemas = PropertySchemas::new();
    schemas.record("SaveData".into(), struct_tag("SyntheticSaveData"));
    schemas.record("SaveData.PlayerUId".into(), guid_tag());
    schemas.record(
        "SaveData.IndividualId".into(),
        struct_tag("SyntheticIndividualId"),
    );
    schemas.record("SaveData.IndividualId.PlayerUId".into(), guid_tag());
    schemas.record("SaveData.IndividualId.InstanceId".into(), guid_tag());
    write(path, root, schemas);
}

fn write_level(path: &std::path::Path, player_uid: &str, opaque_uid: Option<&str>) {
    write_level_with_players(path, &[player_uid], opaque_uid);
}

fn write_level_with_players(
    path: &std::path::Path,
    player_uids: &[&str],
    opaque_uid: Option<&str>,
) {
    let identities: Vec<(&str, &str)> = player_uids
        .iter()
        .map(|player_uid| (*player_uid, INSTANCE))
        .collect();
    write_level_with_identities(path, &identities, opaque_uid, vec![]);
}

fn write_level_with_identities(
    path: &std::path::Path,
    identities: &[(&str, &str)],
    opaque_uid: Option<&str>,
    extra: Vec<u8>,
) {
    let mut world = Properties::default();
    world.insert(
        "CharacterSaveParameterMap",
        Property::Map(
            identities
                .iter()
                .map(|(player_uid, instance_id)| {
                    let mut key = Properties::default();
                    key.insert("PlayerUId", guid(player_uid));
                    key.insert("InstanceId", guid(instance_id));
                    MapEntry {
                        key: Property::Struct(StructValue::Struct(key)),
                        value: Property::Struct(StructValue::Struct(Properties::default())),
                    }
                })
                .collect(),
        ),
    );
    if let Some(opaque_uid) = opaque_uid {
        world.insert(
            "OpaqueBlob",
            Property::Array(ValueVec::Byte(ByteArray::Byte(pal_guid_bytes(opaque_uid)))),
        );
    }
    let mut root = Properties::default();
    root.insert(
        "worldSaveData",
        Property::Struct(StructValue::Struct(world)),
    );

    let mut schemas = PropertySchemas::new();
    schemas.record("worldSaveData".into(), struct_tag("SyntheticWorldSaveData"));
    schemas.record(
        "worldSaveData.CharacterSaveParameterMap".into(),
        PropertyTagPartial {
            id: None,
            data: PropertyTagDataPartial::Map {
                key_type: Box::new(struct_data("SyntheticCharacterKey")),
                value_type: Box::new(struct_data("SyntheticCharacterValue")),
            },
        },
    );
    schemas.record(
        "worldSaveData.CharacterSaveParameterMap.PlayerUId".into(),
        guid_tag(),
    );
    schemas.record(
        "worldSaveData.CharacterSaveParameterMap.InstanceId".into(),
        guid_tag(),
    );
    if opaque_uid.is_some() {
        schemas.record(
            "worldSaveData.OpaqueBlob".into(),
            PropertyTagPartial {
                id: None,
                data: PropertyTagDataPartial::Array(Box::new(PropertyTagDataPartial::Byte(None))),
            },
        );
    }
    write_with_extra(path, root, schemas, extra);
}

fn write(path: &std::path::Path, properties: PalProperties, schemas: PropertySchemas) {
    write_with_extra(path, properties, schemas, vec![]);
}

fn write_with_extra(
    path: &std::path::Path,
    properties: PalProperties,
    schemas: PropertySchemas,
    extra: Vec<u8>,
) {
    let save: Save<Palworld> = Save {
        header: Header {
            magic: u32::from_le_bytes(*b"GVAS"),
            save_game_version: 2,
            package_version: PackageVersion {
                ue4: 522,
                ue5: None,
            },
            engine_version_major: 4,
            engine_version_minor: 27,
            engine_version_patch: 0,
            engine_version_build: 0,
            engine_version: "4.27.0".into(),
            custom_version: Some((3, vec![])),
        },
        schemas,
        root: Root {
            save_game_type: "SyntheticPalworldSave".into(),
            properties,
        },
        extra,
    };
    let mut file = File::create(path).unwrap();
    save.write(&mut file).unwrap();
    file.sync_all().unwrap();
}

fn guid(value: &str) -> PalProperty {
    Property::Struct(StructValue::Guid(FGuid::parse_str(value).unwrap()))
}

fn guid_tag() -> PropertyTagPartial {
    PropertyTagPartial {
        id: None,
        data: PropertyTagDataPartial::Struct {
            struct_type: StructType::Guid,
            id: FGuid::nil(),
        },
    }
}

fn struct_tag(name: &str) -> PropertyTagPartial {
    PropertyTagPartial {
        id: None,
        data: struct_data(name),
    }
}

fn struct_data(name: &str) -> PropertyTagDataPartial {
    PropertyTagDataPartial::Struct {
        struct_type: StructType::Struct(Some(name.to_owned())),
        id: FGuid::nil(),
    }
}

fn player_name(uid: &str) -> String {
    format!("{}.sav", uid.replace('-', "").to_uppercase())
}

fn dps_name(uid: &str) -> String {
    format!("{}_dps.sav", uid.replace('-', "").to_uppercase())
}

fn mapping() -> MappingSet {
    MappingSet::from_json(
        format!(r#"{{"source_uid":"{A}","target_uid":"{B}"}}"#).as_bytes(),
    )
    .unwrap()
}

fn pal_guid_bytes(value: &str) -> Vec<u8> {
    value
        .replace('-', "")
        .as_bytes()
        .chunks_exact(8)
        .flat_map(|chunk| {
            u32::from_str_radix(std::str::from_utf8(chunk).unwrap(), 16)
                .unwrap()
                .to_le_bytes()
        })
        .collect()
}
