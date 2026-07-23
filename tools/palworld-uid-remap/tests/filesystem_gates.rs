use std::fs;
use std::path::Path;
#[cfg(windows)]
use std::process::Command;

use palworld_uid_remap::{remap_world, MappingSet, RemapError, RemapOptions};
use tempfile::TempDir;

const A: &str = "00112233-4455-6677-8899-aabbccddeeff";
const B: &str = "11112233-4455-6677-8899-aabbccddeeff";

fn mapping() -> MappingSet {
    MappingSet::from_json(
        format!(r#"{{"source_uid":"{A}","target_uid":"{B}"}}"#).as_bytes(),
    )
    .unwrap()
}

fn player_name(uid: &str) -> String {
    format!("{}.sav", uid.replace('-', "").to_uppercase())
}

fn world() -> (TempDir, std::path::PathBuf, std::path::PathBuf) {
    let temp = TempDir::new().unwrap();
    let input = temp.path().join("world");
    let output = temp.path().join("output");
    fs::create_dir_all(input.join("Players")).unwrap();
    fs::write(input.join("Level.sav"), b"not-gvas").unwrap();
    fs::write(input.join("Players").join(player_name(A)), b"not-gvas").unwrap();
    (temp, input, output)
}

#[test]
fn rejects_existing_output_before_reading_input_saves() {
    let (_temp, input, output) = world();
    fs::create_dir(&output).unwrap();
    let error = remap_world(&input, &output, &mapping(), &RemapOptions::default()).unwrap_err();
    assert!(matches!(error, RemapError::OutputExists(_)));
}

#[test]
fn rejects_missing_source_and_existing_target_player_files() {
    let (_temp, input, output) = world();
    fs::remove_file(input.join("Players").join(player_name(A))).unwrap();
    let error = remap_world(&input, &output, &mapping(), &RemapOptions::default()).unwrap_err();
    assert!(matches!(error, RemapError::SourcePlayerMissing(_)));

    fs::write(input.join("Players").join(player_name(A)), b"not-gvas").unwrap();
    fs::write(input.join("Players").join(player_name(B)), b"not-gvas").unwrap();
    let error = remap_world(&input, &output, &mapping(), &RemapOptions::default()).unwrap_err();
    assert!(matches!(error, RemapError::TargetPlayerExists(_)));
}

#[test]
fn rejects_noncanonical_player_names_and_parse_failure_leaves_no_stage() {
    let (_temp, input, output) = world();
    fs::write(input.join("Players").join("lowercase000000000000000000000000.sav"), b"x")
        .unwrap();
    let error = remap_world(&input, &output, &mapping(), &RemapOptions::default()).unwrap_err();
    assert!(matches!(error, RemapError::NonCanonicalPlayerFile(_)));

    fs::remove_file(input.join("Players").join("lowercase000000000000000000000000.sav"))
        .unwrap();
    let error = remap_world(&input, &output, &mapping(), &RemapOptions::default()).unwrap_err();
    assert!(matches!(error, RemapError::Parse { .. }), "{error:?}");
    assert!(!output.exists());
    assert_eq!(sibling_stages(&output), 0);
}

#[cfg(windows)]
#[test]
fn rejects_reparse_point_in_input_path_chain_before_canonicalization() {
    let temp = TempDir::new().unwrap();
    let real_parent = temp.path().join("real-parent");
    let real_input = real_parent.join("world");
    fs::create_dir_all(real_input.join("Players")).unwrap();
    fs::write(real_input.join("Level.sav"), b"not-gvas").unwrap();
    fs::write(
        real_input.join("Players").join(player_name(A)),
        b"not-gvas",
    )
    .unwrap();

    let linked_parent = temp.path().join("linked-parent");
    let status = Command::new("cmd")
        .args(["/C", "mklink", "/J"])
        .arg(&linked_parent)
        .arg(&real_parent)
        .status()
        .unwrap();
    assert!(status.success(), "test requires creating a directory junction");

    let error = remap_world(
        linked_parent.join("world"),
        temp.path().join("output"),
        &mapping(),
        &RemapOptions::default(),
    )
    .unwrap_err();
    fs::remove_dir(&linked_parent).unwrap();
    assert!(matches!(error, RemapError::UnsafeEntry(_)), "{error:?}");
}

fn sibling_stages(output: &Path) -> usize {
    output
        .parent()
        .unwrap()
        .read_dir()
        .unwrap()
        .filter_map(Result::ok)
        .filter(|entry| {
            entry
                .file_name()
                .to_string_lossy()
                .starts_with(".output.palworld-uid-remap-stage-")
        })
        .count()
}
