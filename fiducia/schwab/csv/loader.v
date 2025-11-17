// loader.v
module main

import os

const template_basename = 'Brokerage-Positions-TEMPLATE.csv'

// Returns just the "date token" from the latest snapshot, or 'TEMPLATE' if none.
fn get_latest_date_in_dir(data_dir string) string {
	files := os.ls(data_dir) or {
		// On failure, just fall back to template
		return 'TEMPLATE'
	}

	mut dates := []string{}
	for f in files {
		if !f.starts_with('Brokerage-Positions-') || !f.ends_with('.csv') {
			continue
		}
		base := f.all_after('Brokerage-Positions-')
		date_part := base.all_before_last('.csv')
		if date_part == 'TEMPLATE' {
			continue
		}
		dates << date_part
	}

	if dates.len == 0 {
		return 'TEMPLATE'
	}

	dates.sort()
	return dates[dates.len - 1]
}

// resolve_positions_path returns the full path to a positions CSV file.
pub fn resolve_positions_path(data_dir string, date_opt string) !string {
	// If caller supplied a date token, use that; otherwise pick latest.
	date := if date_opt != '' {
		date_opt
	} else {
		get_latest_date_in_dir(data_dir)
	}

	mut path := os.join_path(data_dir, 'Brokerage-Positions-${date}.csv')
	if !os.exists(path) {
		// fall back to template
		path = os.join_path(data_dir, template_basename)
	}

	if !os.exists(path) {
		return error('no matching positions CSV found in ${data_dir} (looked for date "${date}" and template "${template_basename}")')
	}

	return path
}
