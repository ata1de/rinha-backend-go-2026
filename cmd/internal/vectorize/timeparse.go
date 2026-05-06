package vectorize

type fastTS struct {
	year, month, day uint16
	hour, min, sec   uint16
	epochSec         int64
	weekdayMonZero   uint8
}

func parseFastTS(s string) (fastTS, bool) {
	var t fastTS
	if len(s) < 20 || s[4] != '-' || s[7] != '-' || s[10] != 'T' ||
		s[13] != ':' || s[16] != ':' || s[19] != 'Z' {
		return t, false
	}
	y := int(s[0]-'0')*1000 + int(s[1]-'0')*100 + int(s[2]-'0')*10 + int(s[3]-'0')
	mo := int(s[5]-'0')*10 + int(s[6]-'0')
	d := int(s[8]-'0')*10 + int(s[9]-'0')
	h := int(s[11]-'0')*10 + int(s[12]-'0')
	mi := int(s[14]-'0')*10 + int(s[15]-'0')
	se := int(s[17]-'0')*10 + int(s[18]-'0')

	t.year, t.month, t.day = uint16(y), uint16(mo), uint16(d)
	t.hour, t.min, t.sec = uint16(h), uint16(mi), uint16(se)

	days := daysFromCivil(y, mo, d)
	t.epochSec = days*86400 + int64(h)*3600 + int64(mi)*60 + int64(se)

	w := (days%7 + 3) % 7
	if w < 0 {
		w += 7
	}
	t.weekdayMonZero = uint8(w)
	return t, true
}

func daysFromCivil(y, m, d int) int64 {
	if m <= 2 {
		y--
	}
	era := y
	if y < 0 {
		era -= 399
	}
	era /= 400
	yoe := y - era*400 // [0, 399]
	mp := m
	if m > 2 {
		mp -= 3
	} else {
		mp += 9
	}
	doy := (153*mp+2)/5 + d - 1            // [0, 365]
	doe := yoe*365 + yoe/4 - yoe/100 + doy // [0, 146096]
	return int64(era)*146097 + int64(doe) - 719468
}
