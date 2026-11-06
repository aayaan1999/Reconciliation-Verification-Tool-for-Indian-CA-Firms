// Package currency reads the one fixed rate, in rupees per US dollar, that the CA keeps in
// content/currency/usd-rate.yaml (SPEC-019). The file is data: the CA edits it and no code
// changes. The rate is read once when a run starts and is the same for every document in that
// run; no clock is read and no rate is fetched from anywhere. A missing, empty or invalid file
// is not an error for a run with rupee amounts only: the problem is carried in the Rate and
// reported only for a dollar amount that is actually met.
package currency
