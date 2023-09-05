package pgn

func NewPgn127508() *PgnBase {

	pgn := NewPgnBase(127508)

	pgn.Fields = append(pgn.Fields,

		field{
			node: "electrical.battery.voltage",

			instance: func(fields n2kFields) string {
				return fields.Instance()
			},

			filter: func(fields n2kFields) bool {
				return fields.Contains("voltage")
			},

			value: func(fields n2kFields) interface{} {
				return Float64Value(fields["voltage"])
			},
		},

		field{
			node: "electrical.battery.current",

			instance: func(fields n2kFields) string {
				return fields.Instance()
			},

			filter: func(fields n2kFields) bool {
				return fields.Contains("current")
			},

			value: func(fields n2kFields) interface{} {
				return Float64Value(fields["current"])
			},
		},

		field{
			node: "electrical.battery.temperature",

			instance: func(fields n2kFields) string {
				return fields.Instance()
			},

			filter: func(fields n2kFields) bool {
				return fields.Contains("temperature")
			},

			value: func(fields n2kFields) interface{} {
				return Float64Value(fields["temperature"])
			},
		},
	)
	return pgn
}
