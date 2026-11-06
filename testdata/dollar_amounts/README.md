# Dollar amounts (SPEC-019)

Synthetic sample for amounts written in US dollars. Every name and figure is invented.

- `bank_book.csv` and `bank_book.xlsx` (written from the CSV by `go test ./cmd/recon-engine -run TestDollar -update`) hold the same
  three entries; a receipt and a payment are written in dollars (`$1,250.50` and `$80.25`), one receipt in rupees.
- `bank_statement.csv` holds the same three entries in rupees. At the rate 83.2500 rupees per dollar `$1,250.50` is
  Rs 1,04,104.13 (125,050 cents x 832,500 / 10,000 = 10,410,412.5 paise; a half rounds away from zero, so 10,410,413 paise) and
  `$80.25` is Rs 6,680.81 (8,025 x 832,500 / 10,000 = 668,081.25 paise, rounded to 668,081). All three entries agree at 83.2500;
  at 84.0000 the two dollar entries differ.
