// Package strictyaml reads the YAML content files of Tool 1 (vocabulary, profiles, rules,
// law tables) so that nothing in them can be read in more than one way (technical spec
// §B4.1, RU-02). It decodes into typed Go structs and refuses:
//
//   - unknown keys, and duplicate keys;
//   - text that is not in quotes: every Go string field must be a quoted YAML scalar, so
//     `version: 1`, `tax_years: [2025-26]` and `id: RECON-DEP-002` all fail (a block
//     scalar, `|` or `>`, is always text and counts as quoted);
//   - yes/no/on/off and anything else but true or false for a yes/no field;
//   - numbers written as anything but plain decimal digits (no 0x, 1_000, +1 or 1e3);
//   - anchors, aliases, merge keys (<<), explicit tags and more than one document per file.
//
// Struct fields may be strings, bools, ints, pointers, slices, structs and maps with
// string keys (a set of "name: value" lines, such as a rule's inputs). A map's names are
// plain text and must not repeat; its values follow the same rules as everything else.
//
// Every problem in a file is reported, each with file and line, in plain words.
package strictyaml
