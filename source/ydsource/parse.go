package tcpsource

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.einride.tech/can"
)

type CanId struct {
	id       string // original id string
	priority byte   // priority  ((id >> 26) & 0x7)
	source   uint8  // source address 0-255 (id & 0xff)

	dst uint8  // destination
	pgn uint16 //
}

func parseCanId(s string) CanId {
	var result CanId
	result.id = s

	n, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		fmt.Println(err)
	}
	result.priority = uint8((n >> 26) & 0x7)
	result.source = uint8(n & 0xff)

	pf := uint32((n >> 16) & 0xff) // PDU Format
	ps := uint32((n >> 8) & 0xff)  // PDU Specific
	dp := uint32((n >> 24) & 1)    // Data Page

	if pf < 240 {
		/* PDU1 format, the PS contains the destination address */
		result.dst = uint8(ps)
		result.pgn = uint16((dp << 16) + (pf << 8))
	} else {
		/* PDU2 format, the destination is implied global and the PGN is extended */
		result.dst = 0xff
		result.pgn = uint16((dp << 16) + (pf << 8) + ps)
	}

	return result
}

func isYDRAW(s string) bool {
	if s[2] != ':' {
		return false
	}
	if s[13] == 'R' || s[13] == 'T' {
		return true
	} else {
		return false
	}
}

func parse(s string) (can.Frame, error) {
	frame := can.Frame{}
	parts := strings.Split(s, " ")

	//fmt.Println(s);
	//for i:=0; i < len(parts); i++{
	//	fmt.Println(i,parts[i])
	//}

	if len(parts) < 4 {
		return frame, errors.New("invalid Yacht Devices string")
	}

	fid, err := strconv.ParseUint(parts[2], 16, 32)
	if err != nil {
		fmt.Println(parts[2])
		return frame, err
	}
	frame.ID = uint32(fid)
	frame.IsExtended = true

	dataStr := strings.Join(parts[3:], "")
	dataStr = dataStr[:len(dataStr)-2]	// drop \r\n at end of string
	//fmt.Println("dataStr:", dataStr)

	data, err := strconv.ParseUint(dataStr, 16, 64)
	if err != nil {
		return frame, err
	}

	frame.Data.UnpackBigEndian(data)
	frame.Length = 8

	return frame, nil
}
