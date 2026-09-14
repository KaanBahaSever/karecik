package utils

import (
	"time"

	// The zone database, compiled into every binary that imports this package.
	// The time package falls back to it when it cannot find zone files on the
	// system, so Europe/Istanbul resolves on an image that ships no tzdata
	// package — see the runtime stage of the Dockerfile.
	_ "time/tzdata"
)

// Istanbul is the zone the customer menu's price note is written in.
//
// "Fiyatlarımız 14.09.2026 tarihinden itibaren geçerlidir." names a day, and
// which day an instant falls on depends on where it is read. time.Local is only
// the zone this process happens to run in, which is not a property of the menu:
// on a server running in UTC, a price changed between 00:00 and 03:00 Istanbul
// time would print the previous day.
//
// A fixed UTC+3 stands in if the zone ever fails to load. It names the same day
// as the real zone for every instant since 27 March 2016, the last offset change
// the embedded zone database records for Europe/Istanbul: Turkey has kept UTC+3
// all year since then.
var Istanbul = loadIstanbul()

func loadIstanbul() *time.Location {
	location, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		return time.FixedZone("+03", 3*60*60)
	}
	return location
}
