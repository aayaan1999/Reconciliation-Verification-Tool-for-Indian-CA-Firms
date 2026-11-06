-- Migration 0014 (SPEC-019): the fixed dollar rate a run used, so a past run can be explained.
-- usd_rate is rupees per US dollar written with four decimal places ('83.2500'), or '' when the
-- run had no usable rate (then usd_rate_problem says why: 'dollar rate not set' or 'dollar rate
-- not valid'). usd_rate_effective is the date the CA wrote beside the rate (yyyy-mm-dd, or '').
ALTER TABLE runs ADD COLUMN usd_rate           TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN usd_rate_effective TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN usd_rate_problem   TEXT NOT NULL DEFAULT '';

-- A file that holds an amount in US dollars gives different figures at a different rate, so
-- uploading the same file again under another rate is a new draft, not "the same file". read_with
-- names the rate a file was read with ('usd=83.2500', or 'usd=none:dollar rate not set'); it is ''
-- for a file with no dollar amount, whose reading no rate can change.
ALTER TABLE document_versions ADD COLUMN read_with TEXT NOT NULL DEFAULT '';
