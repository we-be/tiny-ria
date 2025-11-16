module main

import encoding.csv

fn main() {
	// date := "2025-11-16-165423"
	date := "TEMPLATE"

	mut parser := csv.new_reader_from_file("./data/Brokerage-Positions-${date}.csv")!
	for {
    items := parser.read() or { break }
    println(items)
	}
}
