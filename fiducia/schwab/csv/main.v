// main.v
module main

import encoding.csv
import os

fn main() {
    // Accept optional date arg, otherwise leave empty (meaning "auto latest")
    date_arg := if os.args.len > 1 { os.args[1] } else { '' }

    data_dir := 'data'
    path := resolve_positions_path(data_dir, date_arg) or {
        eprintln('Error resolving positions CSV: $err')
        return
    }

    println('Using CSV: $path')

    mut parser := csv.new_reader_from_file(path) or {
        eprintln('Failed to open $path: $err')
        return
    }

    for {
        items := parser.read() or { break }
        println(items)
    }
}
