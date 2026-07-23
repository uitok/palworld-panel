use palworld_uid_remap::{MappingError, MappingSet};

const A: &str = "00112233-4455-6677-8899-aabbccddeeff";
const B: &str = "11112233-4455-6677-8899-aabbccddeeff";
const C: &str = "22112233-4455-6677-8899-aabbccddeeff";
const D: &str = "33112233-4455-6677-8899-aabbccddeeff";

fn entry(source: &str, target: &str) -> String {
    format!(r#"{{"source_uid":"{source}","target_uid":"{target}"}}"#)
}

#[test]
fn accepts_single_object_or_array_and_sorts_by_source() {
    let single = MappingSet::from_json(entry(A, B).as_bytes()).unwrap();
    assert_eq!(single.canonical_pairs(), vec![(A.to_owned(), B.to_owned())]);

    let array = format!("[{},{}]", entry(C, D), entry(A, B));
    let parsed = MappingSet::from_json(array.as_bytes()).unwrap();
    assert_eq!(
        parsed.canonical_pairs(),
        vec![(A.to_owned(), B.to_owned()), (C.to_owned(), D.to_owned())]
    );
}

#[test]
fn rejects_empty_mapping() {
    assert_eq!(
        MappingSet::from_json(b"[]").unwrap_err(),
        MappingError::Empty
    );
}

#[test]
fn rejects_malformed_or_noncanonical_guids_and_unknown_fields() {
    for json in [
        r#"{"source_uid":"not-a-guid","target_uid":"11112233-4455-6677-8899-aabbccddeeff"}"#,
        r#"{"source_uid":"00112233445566778899aabbccddeeff","target_uid":"11112233-4455-6677-8899-aabbccddeeff"}"#,
        r#"{"source_uid":"00112233-4455-6677-8899-AABBCCDDEEFF","target_uid":"11112233-4455-6677-8899-aabbccddeeff"}"#,
        r#"{"source_uid":"00112233-4455-6677-8899-aabbccddeeff","target_uid":"11112233-4455-6677-8899-aabbccddeeff","extra":true}"#,
    ] {
        assert!(MappingSet::from_json(json.as_bytes()).is_err(), "accepted {json}");
    }
}

#[test]
fn rejects_duplicate_sources_and_targets() {
    let duplicate_source = format!("[{},{}]", entry(A, B), entry(A, C));
    assert_eq!(
        MappingSet::from_json(duplicate_source.as_bytes()).unwrap_err(),
        MappingError::DuplicateSource(A.to_owned())
    );

    let duplicate_target = format!("[{},{}]", entry(A, C), entry(B, C));
    assert_eq!(
        MappingSet::from_json(duplicate_target.as_bytes()).unwrap_err(),
        MappingError::DuplicateTarget(C.to_owned())
    );
}

#[test]
fn rejects_self_mapping_and_source_target_intersection() {
    assert_eq!(
        MappingSet::from_json(entry(A, A).as_bytes()).unwrap_err(),
        MappingError::SelfMapping(A.to_owned())
    );

    let intersecting = format!("[{},{}]", entry(A, B), entry(C, A));
    assert_eq!(
        MappingSet::from_json(intersecting.as_bytes()).unwrap_err(),
        MappingError::SourceTargetIntersection(A.to_owned())
    );
}
