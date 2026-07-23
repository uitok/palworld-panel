use std::collections::{BTreeMap, BTreeSet};

use serde::Serialize;
use uesave::games::palworld::{
    PalGroupVariant, PalGuildTail, PalMapConcreteModelModuleData, PalMapConcreteModelVariant,
    PalStruct, Palworld,
};
use uesave::{
    ByteArray, FGuid, Properties, Property, SaveGameArchiveType, StructValue, ValueVec,
};

use crate::MappingSet;

type PalProperty = Property<SaveGameArchiveType<Palworld>>;
type PalProperties = Properties<SaveGameArchiveType<Palworld>>;

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum FieldCategory {
    PlayerSave,
    CharacterIndex,
    Guild,
    PalOwnership,
    BuildOwnership,
    MapOwnership,
    Lock,
    Trading,
    AdditionalPlayerUid,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum CandidateKind {
    Source,
    Target,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct OpaqueCandidate {
    pub file: String,
    pub path: String,
    pub uid: String,
    pub kind: CandidateKind,
    pub offset: usize,
}

#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize)]
pub struct RewriteReport {
    pub rewritten_fields: BTreeMap<FieldCategory, u64>,
    pub rewritten_by_source: BTreeMap<String, BTreeMap<FieldCategory, u64>>,
    pub opaque_candidates: Vec<OpaqueCandidate>,
}

impl RewriteReport {
    pub fn count(&self, category: FieldCategory) -> u64 {
        self.rewritten_fields.get(&category).copied().unwrap_or(0)
    }

    fn rewritten(&mut self, source: &FGuid, category: FieldCategory) {
        *self.rewritten_fields.entry(category).or_default() += 1;
        *self
            .rewritten_by_source
            .entry(source.to_string())
            .or_default()
            .entry(category)
            .or_default() += 1;
    }
}

pub fn rewrite_typed_tree(
    properties: &mut PalProperties,
    mapping: &MappingSet,
    file: &str,
) -> RewriteReport {
    let file_kind = SaveFileKind::from_relative(file);
    let player_instances = collect_player_instances(properties, file_kind);
    let mut visitor = Visitor {
        mapping,
        file,
        file_kind,
        player_instances,
        report: RewriteReport::default(),
    };
    visitor.properties(properties, "");
    visitor.report
}

pub(crate) fn scan_opaque_bytes(
    bytes: &[u8],
    mapping: &MappingSet,
    file: &str,
    path: &str,
) -> RewriteReport {
    let mut visitor = Visitor {
        mapping,
        file,
        file_kind: SaveFileKind::from_relative(file),
        player_instances: BTreeMap::new(),
        report: RewriteReport::default(),
    };
    visitor.scan(bytes, path);
    visitor.report
}

struct Visitor<'a> {
    mapping: &'a MappingSet,
    file: &'a str,
    file_kind: SaveFileKind,
    player_instances: BTreeMap<FGuid, BTreeSet<FGuid>>,
    report: RewriteReport,
}

impl Visitor<'_> {
    fn properties(&mut self, properties: &mut PalProperties, prefix: &str) {
        for (key, property) in &mut properties.0 {
            let path = join(prefix, &key.1);
            self.property(
                property,
                &path,
                category_for_path(self.file_kind, &path),
            );
        }
    }

    fn property(
        &mut self,
        property: &mut PalProperty,
        path: &str,
        category: Option<FieldCategory>,
    ) {
        match property {
            Property::Struct(value) => self.struct_value(value, path, category),
            Property::Map(entries) => {
                for (index, entry) in entries.iter_mut().enumerate() {
                    self.property(
                        &mut entry.key,
                        &format!("{path}[{index}].Key"),
                        category,
                    );
                    self.property(
                        &mut entry.value,
                        &format!("{path}[{index}].Value"),
                        category,
                    );
                }
            }
            Property::Array(values) | Property::Set(values) => {
                self.values(values, path, category)
            }
            Property::Raw(bytes) => self.scan(bytes, path),
            _ => {}
        }
    }

    fn values(&mut self, values: &mut ValueVec<SaveGameArchiveType<Palworld>>, path: &str, category: Option<FieldCategory>) {
        match values {
            ValueVec::Struct(values) => {
                for (index, value) in values.iter_mut().enumerate() {
                    self.struct_value(value, &format!("{path}[{index}]"), category);
                }
            }
            ValueVec::Byte(ByteArray::Byte(bytes)) => self.scan(bytes, path),
            _ => {}
        }
    }

    fn struct_value(
        &mut self,
        value: &mut StructValue<SaveGameArchiveType<Palworld>>,
        path: &str,
        category: Option<FieldCategory>,
    ) {
        match value {
            StructValue::Guid(uid) => {
                if let Some(category) = category {
                    self.replace(uid, category);
                }
            }
            StructValue::Struct(properties) => self.properties(properties, path),
            StructValue::Game(value) => self.pal_struct(value, path),
            StructValue::Raw(bytes) => self.scan(bytes, path),
            _ => {}
        }
    }

    fn pal_struct(
        &mut self,
        value: &mut PalStruct<SaveGameArchiveType<Palworld>>,
        path: &str,
    ) {
        match value {
            PalStruct::CharacterData(value) => {
                self.properties(&mut value.object, &join(path, "object"));
                self.scan(&value.unknown_bytes, &join(path, "unknown_bytes"));
                self.scan(&value.trailing_bytes, &join(path, "trailing_bytes"));
            }
            PalStruct::CharacterContainer(value) => {
                self.replace(&mut value.player_uid, FieldCategory::CharacterIndex);
                if let Some(bytes) = &value.trailing_bytes {
                    self.scan(bytes, &join(path, "trailing_bytes"));
                }
            }
            PalStruct::GroupData(group) => {
                for handle in &mut group.individual_character_handle_ids {
                    let tied_to_player = self
                        .player_instances
                        .get(&handle.guid)
                        .is_some_and(|instances| instances.contains(&handle.instance_id));
                    if tied_to_player {
                        self.replace(&mut handle.guid, FieldCategory::Guild);
                    }
                }
                match &mut group.data {
                    PalGroupVariant::Guild(guild) => {
                        self.replace(
                            &mut guild.last_guild_name_modifier_player_uid,
                            FieldCategory::Guild,
                        );
                        for marker in &mut guild.guild_markers {
                            self.replace(&mut marker.owner_player_uid, FieldCategory::Guild);
                        }
                        match &mut guild.tail {
                            PalGuildTail::PreUpdate(tail) => {
                                self.replace(&mut tail.admin_player_uid, FieldCategory::Guild);
                                for player in &mut tail.players {
                                    self.replace(&mut player.player_uid, FieldCategory::Guild);
                                }
                                self.scan(&tail.trailing_bytes, &join(path, "tail.trailing_bytes"));
                            }
                            PalGuildTail::PostUpdate(tail) => {
                                self.replace(&mut tail.admin_player_uid, FieldCategory::Guild);
                                for player in &mut tail.players {
                                    self.replace(&mut player.player_uid, FieldCategory::Guild);
                                }
                                self.scan(&tail.trailing_bytes, &join(path, "tail.trailing_bytes"));
                            }
                        }
                        self.scan(&guild.leading_bytes, &join(path, "leading_bytes"));
                    }
                    PalGroupVariant::IndependentGuild(guild) => {
                        self.replace(&mut guild.player_uid, FieldCategory::Guild);
                    }
                    PalGroupVariant::Unknown { remaining_data } => {
                        self.scan(remaining_data, &join(path, "remaining_data"));
                    }
                    PalGroupVariant::Organization(group) => {
                        self.scan(&group.trailing_bytes, &join(path, "trailing_bytes"));
                    }
                }
            }
            PalStruct::MapModel(model) => {
                self.replace(&mut model.build_player_uid, FieldCategory::BuildOwnership);
                self.scan(&model.unknown_bytes, &join(path, "unknown_bytes"));
            }
            PalStruct::MapConcreteModel(model) => self.concrete_model(&mut model.model_data, path),
            PalStruct::MapConcreteModelModule(module) => match &mut module.data {
                PalMapConcreteModelModuleData::PasswordLock {
                    player_infos,
                    trailing_bytes,
                    ..
                } => {
                    for info in player_infos {
                        self.replace(&mut info.player_uid, FieldCategory::Lock);
                    }
                    self.scan(trailing_bytes, &join(path, "trailing_bytes"));
                }
                PalMapConcreteModelModuleData::Unknown { raw_bytes, .. } => {
                    self.scan(raw_bytes, &join(path, "raw_bytes"));
                }
                _ => {}
            },
            PalStruct::DynamicItem(value) => {
                if let uesave::games::palworld::PalDynamicItemType::Unknown { trailer } =
                    &value.item_type
                {
                    self.scan(trailer, &join(path, "trailer"));
                }
            }
            _ => {}
        }
    }

    fn concrete_model(
        &mut self,
        model: &mut PalMapConcreteModelVariant<SaveGameArchiveType<Palworld>>,
        path: &str,
    ) {
        match model {
            PalMapConcreteModelVariant::ItemBooth(model) => {
                self.replace(&mut model.private_lock_player_uid, FieldCategory::Lock);
                for trade in &mut model.trade_infos {
                    self.replace(&mut trade.seller_player_uid, FieldCategory::Trading);
                }
                self.scan(&model.trailing_bytes, &join(path, "trailing_bytes"));
            }
            PalMapConcreteModelVariant::DeathDroppedCharacter(model) => {
                self.replace(&mut model.owner_player_uid, FieldCategory::MapOwnership);
                self.scan(&model.trailing_bytes, &join(path, "trailing_bytes"));
            }
            PalMapConcreteModelVariant::DropItem(model) => {
                self.replace(&mut model.pickupable_player_uid, FieldCategory::MapOwnership);
                self.scan(&model.trailing_bytes, &join(path, "trailing_bytes"));
            }
            PalMapConcreteModelVariant::DeathPenaltyStorage(model) => {
                self.replace(&mut model.owner_player_uid, FieldCategory::MapOwnership);
                self.scan(&model.trailing_bytes, &join(path, "trailing_bytes"));
            }
            PalMapConcreteModelVariant::Signboard(model) => {
                self.replace(
                    &mut model.last_modified_player_uid,
                    FieldCategory::AdditionalPlayerUid,
                );
                self.scan(&model.trailing_bytes, &join(path, "trailing_bytes"));
            }
            PalMapConcreteModelVariant::PalEgg(model) => {
                self.replace(
                    &mut model.pickupdable_player_uid,
                    FieldCategory::MapOwnership,
                );
            }
            PalMapConcreteModelVariant::ItemChest(model) => {
                self.replace(&mut model.private_lock_player_uid, FieldCategory::Lock);
                self.scan(&model.trailing_bytes, &join(path, "trailing_bytes"));
            }
            PalMapConcreteModelVariant::ItemChestAffectCorruption(model) => {
                self.replace(&mut model.private_lock_player_uid, FieldCategory::Lock);
                self.scan(&model.trailing_bytes, &join(path, "trailing_bytes"));
            }
            PalMapConcreteModelVariant::HatchingEgg(model) => {
                self.properties(
                    &mut model.hatched_character_save_parameter,
                    &join(path, "hatched_character_save_parameter"),
                );
            }
            PalMapConcreteModelVariant::Unknown(model) => {
                self.scan(&model.trailing_bytes, &join(path, "trailing_bytes"));
            }
            _ => {}
        }
    }

    fn replace(&mut self, uid: &mut FGuid, category: FieldCategory) {
        let source = *uid;
        if let Some(target) = self.mapping.replacement(uid) {
            *uid = target;
            self.report.rewritten(&source, category);
        }
    }

    fn scan(&mut self, bytes: &[u8], path: &str) {
        for (source, target) in self.mapping.pairs() {
            self.scan_uid(bytes, path, source, CandidateKind::Source);
            self.scan_uid(bytes, path, target, CandidateKind::Target);
        }
    }

    fn scan_uid(&mut self, bytes: &[u8], path: &str, uid: &FGuid, kind: CandidateKind) {
        let needle = guid_bytes(uid);
        for (offset, window) in bytes.windows(needle.len()).enumerate() {
            if window == needle {
                self.report.opaque_candidates.push(OpaqueCandidate {
                    file: self.file.to_owned(),
                    path: path.to_owned(),
                    uid: uid.to_string(),
                    kind,
                    offset,
                });
            }
        }
    }
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum SaveFileKind {
    Level,
    Player,
    Other,
}

impl SaveFileKind {
    fn from_relative(file: &str) -> Self {
        let file = file.replace('\\', "/");
        if file == "Level.sav" {
            Self::Level
        } else if file.starts_with("Players/") && file.ends_with(".sav") {
            Self::Player
        } else {
            Self::Other
        }
    }
}

fn category_for_path(file_kind: SaveFileKind, path: &str) -> Option<FieldCategory> {
    let tokens: Vec<&str> = path
        .split('.')
        .map(|token| token.split_once('[').map_or(token, |(name, _)| name))
        .collect();
    if file_kind == SaveFileKind::Player
        && (tokens == ["SaveData", "PlayerUId"]
            || tokens == ["SaveData", "IndividualId", "PlayerUId"])
    {
        return Some(FieldCategory::PlayerSave);
    }
    if file_kind != SaveFileKind::Level {
        return None;
    }
    if tokens == ["CharacterSaveParameterMap", "Key", "PlayerUId"]
        || tokens
            == [
                "worldSaveData",
                "CharacterSaveParameterMap",
                "Key",
                "PlayerUId",
            ]
    {
        return Some(FieldCategory::CharacterIndex);
    }

    match tokens.last().copied() {
        Some("OwnerPlayerUId" | "OldOwnerPlayerUIds") => Some(FieldCategory::PalOwnership),
        Some("build_player_uid") => Some(FieldCategory::BuildOwnership),
        Some("owner_player_uid" | "pickupable_player_uid" | "pickupdable_player_uid") => {
            Some(FieldCategory::MapOwnership)
        }
        Some("private_lock_player_uid") => Some(FieldCategory::Lock),
        Some("seller_player_uid") => Some(FieldCategory::Trading),
        Some("admin_player_uid" | "last_guild_name_modifier_player_uid") => {
            Some(FieldCategory::Guild)
        }
        _ => None,
    }
}

fn collect_player_instances(
    properties: &PalProperties,
    file_kind: SaveFileKind,
) -> BTreeMap<FGuid, BTreeSet<FGuid>> {
    let mut result = BTreeMap::new();
    match file_kind {
        SaveFileKind::Player => {
            let Some(save_data) = named_struct(properties, "SaveData") else {
                return result;
            };
            let Some(individual) = named_struct(save_data, "IndividualId") else {
                return result;
            };
            record_instance_pair(individual, &mut result);
        }
        SaveFileKind::Level => {
            if let Some(character_map) = named_property(properties, "CharacterSaveParameterMap") {
                collect_character_map_instances(character_map, &mut result);
            }
            if let Some(world) = named_struct(properties, "worldSaveData") {
                if let Some(character_map) = named_property(world, "CharacterSaveParameterMap") {
                    collect_character_map_instances(character_map, &mut result);
                }
            }
        }
        SaveFileKind::Other => {}
    }
    result
}

fn collect_character_map_instances(
    property: &PalProperty,
    result: &mut BTreeMap<FGuid, BTreeSet<FGuid>>,
) {
    let Property::Map(entries) = property else {
        return;
    };
    for entry in entries {
        if let Property::Struct(StructValue::Struct(key)) = &entry.key {
            record_instance_pair(key, result);
        }
    }
}

fn record_instance_pair(
    properties: &PalProperties,
    result: &mut BTreeMap<FGuid, BTreeSet<FGuid>>,
) {
    let Some(player_uid) = named_guid(properties, "PlayerUId") else {
        return;
    };
    let Some(instance_id) = named_guid(properties, "InstanceId") else {
        return;
    };
    result.entry(player_uid).or_default().insert(instance_id);
}

fn named_property<'a>(properties: &'a PalProperties, name: &str) -> Option<&'a PalProperty> {
    properties.0.get(&uesave::PropertyKey::from(name))
}

fn named_struct<'a>(properties: &'a PalProperties, name: &str) -> Option<&'a PalProperties> {
    match named_property(properties, name) {
        Some(Property::Struct(StructValue::Struct(value))) => Some(value),
        _ => None,
    }
}

fn named_guid(properties: &PalProperties, name: &str) -> Option<FGuid> {
    match named_property(properties, name) {
        Some(Property::Struct(StructValue::Guid(value))) => Some(*value),
        _ => None,
    }
}

fn guid_bytes(uid: &FGuid) -> [u8; 16] {
    let compact = uid.to_string().replace('-', "");
    let mut result = [0; 16];
    for (index, chunk) in compact.as_bytes().chunks_exact(8).enumerate() {
        let value = u32::from_str_radix(std::str::from_utf8(chunk).expect("GUID is ASCII"), 16)
            .expect("FGuid display is hexadecimal");
        result[index * 4..index * 4 + 4].copy_from_slice(&value.to_le_bytes());
    }
    result
}

fn join(prefix: &str, name: &str) -> String {
    if prefix.is_empty() {
        name.to_owned()
    } else {
        format!("{prefix}.{name}")
    }
}
