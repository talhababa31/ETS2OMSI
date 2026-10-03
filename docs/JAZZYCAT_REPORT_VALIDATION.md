# Jazzycat v2.9 scan-report validation

Validation input: the user's `ets2omsi_scan 2.json` generated from `ai_traffic_pack_by_Jazzycat_v2.9.scs`.

Observed car roots: 171 storage-bound cars.

Before the V2 bridge:
- 24 cars already carried resolved PMD model sets.
- 147 car roots did not yet carry model sets.

V2 bridge simulation against that report:
- 142 unresolved cars map uniquely by exact traffic/chassis unit suffix.
- 5 remaining cars map uniquely by the root-definition directory name:
  - Bentley Arnage
  - Dodge Grand Caravan
  - Ford Scorpio
  - Mercedes Sprinter 903
  - Renault Master Cargo
- Result: 171 / 171 storage-bound car roots have a deterministic package-local model-set match in the supplied report.

This validates the scanner/linking strategy only. Actual PMD/PMG decoding and OMSI rendering still require real model conversion on Windows and in-game testing.
