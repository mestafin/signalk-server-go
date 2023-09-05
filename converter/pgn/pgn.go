package pgn

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/wdantuma/signalk-server-go/canboat"
	"github.com/wdantuma/signalk-server-go/ref"
	"github.com/wdantuma/signalk-server-go/signalk"
	"github.com/wdantuma/signalk-server-go/signalkserver/state"
	"github.com/wdantuma/signalk-server-go/source"
	"go.einride.tech/can"
)

type n2kFields map[string]interface{}

func (field n2kFields) Contains(key string) bool {
	_, ok := field[key]
	return ok
}

func (field n2kFields) Instance() string {
	inst, ok := field["instance"]
	if ok {
		return fmt.Sprintf("%.0f", Float64Value(inst))
	} else {
		return ""
	}
}

type field struct {
	filter   func(n2kFields) bool
	value    func(n2kFields) interface{}
	context  func(n2kFields) *string
	instance func(n2kFields) string
	node     string
	source   string
}

type PgnBase struct {
	Pgn     uint
	PgnInfo *canboat.PGNInfo
	Canboat *canboat.Canboat
	Fields  []field
	State   state.ServerState
}

type Pgn interface {
	Convert(can.Frame, source.CanSource) (signalk.DeltaJson, bool)
}

func NewPgnBase(pgn uint) *PgnBase {
	return &PgnBase{Pgn: pgn, Fields: make([]field, 0)}

}

func (base *PgnBase) getDelta(frame source.ExtendedFrame, source string) signalk.DeltaJson {
	src := frame.ID & 0xFF
	delta := signalk.DeltaJson{}
	delta.Context = ref.String(base.State.GetSelf())
	update := signalk.DeltaJsonUpdatesElem{}
	update.Timestamp = ref.UTCTimeStamp(time.Now()) // TODO get from source
	update.Source = &signalk.Source{
		Pgn:   ref.Float64(float64(base.Pgn)),
		Src:   ref.String(strconv.FormatUint(uint64(src), 10)),
		Type:  "NMEA2000",
		Label: source,
	}

	//update.Values = pgnConverter.Convert(update.Values)
	delta.Updates = append(delta.Updates, update)
	return delta
}

func (pgn *PgnBase) Convert(frame source.ExtendedFrame, source string) (signalk.DeltaJson, bool) {
	delta := pgn.getDelta(frame, source)

	lookupFieldTypeField := canboat.Field{}

	fields := make(n2kFields)
	metadata := make(map[string]signalk.Meta)

	//log.Println("\nPgn:", pgn.Pgn) // print PGN number

	// iterate PGN field definitions and update 'fields' of type n2kfields
	// with the value of the field from the source frame
	for _, f := range pgn.PgnInfo.Fields.Field {

		pgnField := f // copy

		meta := signalk.Meta{}
		unit := f.Unit
		meta.Units = &unit
		meta.Description = f.Description
		metadata[f.Id] = meta

		if pgnField.BitOffset == 0 && pgnField.BitLength == 0 {
			pgnField.BitOffset = lookupFieldTypeField.BitOffset
			pgnField.BitLength = lookupFieldTypeField.BitLength
			pgnField.FieldType = lookupFieldTypeField.FieldType
			pgnField.Unit = lookupFieldTypeField.Unit
			pgnField.Signed = lookupFieldTypeField.Signed
			pgnField.Resolution = lookupFieldTypeField.Resolution
			pgnField.RangeMax = lookupFieldTypeField.RangeMax
			pgnField.RangeMin = lookupFieldTypeField.RangeMin
			pgnField.LookupEnumeration = lookupFieldTypeField.LookupEnumeration
			pgnField.LookupBitEnumeration = lookupFieldTypeField.LookupBitEnumeration
		} else {
			lookupFieldTypeField.BitOffset = pgnField.BitOffset + pgnField.BitLength
		}

		switch pgnField.FieldType {

		case "LOOKUP":
			value := float64(frame.UnsignedBitsLittleEndian(int(pgnField.BitOffset), int(pgnField.BitLength))) * float64(pgnField.Resolution)
			if value >= float64(pgnField.RangeMin) && value <= float64(pgnField.RangeMax) {
				refValue, ok := pgn.Canboat.GetLookupEnumeration(pgnField.LookupEnumeration, Float64Value(value))
				if ok {
					fields[pgnField.Id] = refValue
				}

				fieldType, ok := pgn.Canboat.GetLookupFieldTypeEnumeration(pgnField.LookupFieldTypeEnumeration, Float64Value(value))
				if ok {
					fields[pgnField.Id] = fieldType.Name
					lookupFieldTypeField.FieldType = fieldType.FieldType
					lookupFieldTypeField.Signed = fieldType.Signed
					lookupFieldTypeField.Unit = fieldType.Unit
					lookupFieldTypeField.Resolution = pgnField.Resolution
					lookupFieldTypeField.BitLength = fieldType.Bits
					lookupFieldTypeField.RangeMax = 255 // TODO Fix this
					if fieldType.FieldType == "LOOKUP" {
						lookupFieldTypeField.LookupEnumeration = fieldType.LookupEnumeration
					}
				}

			}

		case "NUMBER":
			var value float64
			if pgnField.Signed {
				value = float64(frame.SignedBitsLittleEndian(int(pgnField.BitOffset), int(pgnField.BitLength))) * float64(pgnField.Resolution)
			} else {
				value = float64(frame.UnsignedBitsLittleEndian(int(pgnField.BitOffset), int(pgnField.BitLength))) * float64(pgnField.Resolution)
			}

			if pgn.State.GetDebug() {
				// do not filter out of limit values
				fields[pgnField.Id] = value
			} else {
				if value >= float64(pgnField.RangeMin) && value <= float64(pgnField.RangeMax) {
					fields[pgnField.Id] = value
				}
			}

		case "MMSI":
			var value float64
			value = float64(frame.UnsignedBitsLittleEndian(int(pgnField.BitOffset), int(pgnField.BitLength))) * float64(pgnField.Resolution)
			if value >= float64(pgnField.RangeMin) && value <= float64(pgnField.RangeMax) {
				fields[pgnField.Id] = fmt.Sprintf("%.0f", value)
			}
		}
	}

	var include bool = false
	for _, field := range pgn.Fields {

		val := signalk.DeltaJsonUpdatesElemValuesElem{}
		meta := signalk.DeltaJsonUpdatesElemMetaElem{}

		if field.context != nil {
			delta.Context = field.context(fields)
		} else {

			// add instance to node path
			if field.instance != nil {
				instance := field.instance(fields)
				field.node = addInstance(field.node, instance)
			}

			val.Path = field.node
			meta.Path = field.node

			if field.source != "" {
				value, ok := fields[field.source]
				if !ok {
					continue
				}
				m, ok := metadata[field.source]
				if ok {
					meta.Value = m
				}
				val.Value = value
			} else if field.value != nil {
				val.Value = field.value(fields)
			} else {
				log.Println("No value function")
				continue
			}

			if (field.filter != nil && field.filter(fields)) || field.filter == nil {
				include = true
				delta.Updates[len(delta.Updates)-1].Meta = append(delta.Updates[len(delta.Updates)-1].Meta, meta)
				delta.Updates[len(delta.Updates)-1].Values = append(delta.Updates[len(delta.Updates)-1].Values, val)
			}

		}
	}

	return delta, include
}

func (base *PgnBase) Init(canboat *canboat.Canboat, state state.ServerState) bool {
	base.Canboat = canboat
	base.State = state
	pgnInfo, ok := canboat.GetPGNInfo(base.Pgn)
	if !ok {
		return false
	}
	base.PgnInfo = pgnInfo
	return true
}

func (pgn *PgnBase) GetInstance() *string {
	//pgn.PgnInfo.Fields
	return nil
}

func Float64Value(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	default:
		return 0
	}
}

func StringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func MapValue(value interface{}) map[string]interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return v
	default:
		return nil
	}
}

func GetMmsiContext(fields n2kFields) *string {
	if fields.Contains("userId") {
		mmsi := fmt.Sprintf("vessels.urn:mrn:imo:mmsi:%s", StringValue(fields["userId"]))
		return &mmsi
	} else {
		return nil
	}
}

func addInstance(node string, instance string) string {
	// replace last dot in node string with instance
	if instance != "" {
		i := strings.LastIndex(node, ".")
		return node[:i] + "." + instance + node[i:]
	}
	return node
}

func skEngineId(fields n2kFields) *string {

	inst, ok := fields["instance"]
	var id string
	if ok {
		if reflect.TypeOf(inst).String() == "int" {
			id = fmt.Sprintf("%.0f", Float64Value(inst))
		} else {
			if inst == "Single Engine or Dual Engine Port" {
				id = "port"
			} else {
				id = "starboard"
			}
		}
		return &id
	} else {
		return nil
	}
}
