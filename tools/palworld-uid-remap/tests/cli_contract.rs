use std::process::Command;

#[test]
fn requires_explicit_input_output_and_mapping_arguments() {
    let output = Command::new(env!("CARGO_BIN_EXE_palworld-uid-remap"))
        .output()
        .unwrap();
    assert!(!output.status.success());
    let stderr = String::from_utf8(output.stderr).unwrap();
    for required in ["--input", "--output", "--mapping"] {
        assert!(stderr.contains(required), "missing {required}: {stderr}");
    }
}
