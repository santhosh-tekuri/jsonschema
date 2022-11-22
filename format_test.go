package jsonschema

import (
	"strings"
	"testing"
)

type test struct {
	str   string
	valid bool
}

func TestFormatsNonString(t *testing.T) {
	for name, check := range Formats {
		if !check(1) {
			t.Errorf("%s: want true, got false", name)
		}
	}
}

func TestIsDateTime(t *testing.T) {
	tests := []test{
		{"1963-06-19T08:30:06.283185Z", true},    // with second fraction
		{"1963-06-19T08:30:06Z", true},           // without second fraction
		{"1937-01-01T12:00:27.87+00:20", true},   // with plus offset
		{"1990-12-31T15:59:50.123-08:00", true},  // with minus offset
		{"1990-02-31T15:59:60.123-08:00", false}, // invalid day
		{"1990-12-31T15:59:60-24:00", false},     // invalid offset
		{"06/19/1963 08:30:06 PST", false},       // invalid date delimiters
		{"1963-06-19t08:30:06.283185z", true},    // case-insensitive T and Z
		{"2013-350T01:01:01", false},             // invalid: only RFC3339 not all of ISO 8601 are valid
		{"1963-6-19T08:30:06.283185Z", false},    // invalid: non-padded month
		{"1963-06-1T08:30:06.283185Z", false},    // invalid: non-padded day
		{"1985-04-12T23:20:50.52Z", true},
		{"1996-12-19T16:39:57-08:00", true},
		{"1990-12-31T23:59:59Z", true},
		{"1990-12-31T15:59:59-08:00", true},
	}
	for i, test := range tests {
		if test.valid != isDateTime(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsDate(t *testing.T) {
	tests := []test{
		{"1963-06-19", true},
		{"2020-01-31", true},  // valid: 31 days in January
		{"2020-01-32", false}, // invalid: 32 days in January
		{"2021-02-28", true},  // valid: 28 days in February (normal)
		{"2021-02-29", false}, // invalid: 29 days in February (normal)
		{"2020-02-29", true},  // valid: 29 days in February (leap)
		{"2020-02-30", false}, // invalid: 30 days in February (leap)
		{"2020-03-31", true},  // valid: 31 days in March
		{"2020-03-32", false}, // invalid: 32 days in March
		{"2020-04-30", true},  // valid: 30 days in April
		{"2020-04-31", false}, // invalid: 31 days in April
		{"2020-05-31", true},  // valid: 31 days in May
		{"2020-05-32", false}, // invalid: 32 days in May
		{"2020-06-30", true},  // valid: 30 days in June
		{"2020-06-31", false}, // invalid: 31 days in June
		{"2020-07-31", true},  // valid: 31 days in July
		{"2020-07-32", false}, // invalid: 32 days in July
		{"2020-08-31", true},  // valid: 31 days in August
		{"2020-08-32", false}, // invalid: 32 days in August
		{"2020-09-30", true},  // valid: 30 days in September
		{"2020-09-31", false}, // invalid: 31 days in September
		{"2020-10-31", true},  // valid: 31 days in October
		{"2020-10-32", false}, // invalid: 32 days in October
		{"2020-11-30", true},  // valid: 30 days in November
		{"2020-11-31", false}, // invalid: 31 days in November
		{"2020-12-31", true},  // valid: 31 days in December
		{"2020-12-32", false}, // invalid: 32 days in December
		{"2020-13-01", false}, // invalid month
		{"06/19/1963", false}, // invalid: wrong delimiters
		{"2013-350", false},   // invalid: only RFC3339 not all of ISO 8601 are valid
		{"1998-1-20", false},  // invalid: non-padded month
		{"1998-01-1", false},  // invalid: non-padded day
	}
	for i, test := range tests {
		if test.valid != isDate(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsTime(t *testing.T) {
	tests := []test{
		{"08:30:06.283185Z", true},
		{"08:30:06 PST", false},
		{"01:01:01,1111", false},   // only RFC3339 not all of ISO 8601 are valid
		{"23:59:60Z", true},        // with leap second
		{"15:59:60-08:00", true},   // with leap second with offset
		{"23:20:50.52Z", true},     // with second fraction
		{"08:30:06.283185Z", true}, // with precise second fraction
		{"23:20:50.Z", false},      // invalid (no digit after dot in second fraction)
		{"08:30:06+00:20", true},   // with plus offset
		{"08:30:06-08:00", true},   // with minus offset
		{"08:30:06z", true},        // with case-insensitive Z
		{"24:00:00Z", false},       // invalid hour
		{"00:60:00Z", false},       // invalid minute
		{"00:00:61Z", false},       // invalid second
		{"22:59:60Z", false},       // invalid leap second (wrong hour)
		{"23:58:60Z", false},       // invalid leap second (wrong minute)
		{"01:02:03+24:00", false},  // invalid time numoffset hour
		{"01:02:03+00:60", false},  // invalid time numoffset minute
		{"01:02:03Z+00:30", false}, // invalid time with both Z and numoffset
		{"01:29:60+01:30", true},   // leap second, positive time-offset
		{"12:00:00.52", false},     // no time offset with second fraction
		{"1২:00:00Z", false},       // invalid non-ASCII '২' (a Bengali 2)
		{"08:30:06#00:20", false},  // offset not starting with plus or minus
		{"ab:cd:efz", false},       // contains letters
	}
	for i, test := range tests {
		if test.valid != isTime(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsDuration(t *testing.T) {
	tests := []test{
		{"P4DT12H30M5S", true},
		{"PT1D", false},   // invalid: days after 'T'
		{"P", false},      // invalid: no elements
		{"P1YT", false},   // invalid: no time elements after 'T'
		{"PT", false},     // invalid: no date or time elements
		{"P2D1Y", false},  // invalid: elements out of order
		{"P1D2H", false},  // invalid: missing time separator
		{"P2S", false},    // invalid: time element in the date position
		{"P4Y", true},     // valid: four years duration
		{"PT0S", true},    // valid: zero time, in seconds
		{"P0D", true},     // valid: zero time, in days
		{"P1M", true},     // valid: one month duration
		{"PT1M", true},    // valid: one minute duration
		{"PT36H", true},   // valid: one and a half days, in hours
		{"P1DT12H", true}, // valid: one and a half days, in days and hours
		{"P2W", true},     // valid: two weeks
		{"P1Y2W", false},  // invalid: weeks cannot be combined with other units
		{"P1", false},     // element without unit
	}
	for i, test := range tests {
		if test.valid != isDuration(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsPeriod(t *testing.T) {
	dt := "1963-06-19T08:30:06Z"
	dur := "P4DT12H30M5S"
	tests := []test{
		{dt + "/" + dt, true},  // period-explicit
		{dt + "/" + dur, true}, // period-start
		{dur + "/" + dt, true}, // period-end
		{dur + "/" + dur, false},
		{dt, false},
		{dur, false},
		{dt + "/" + dt + "/" + dt, false},
		{dt + " " + dt, false},
		{dt + "-" + dt, false},
		{"foo/bar", false},
		{"", false},
		{"/", false},
	}
	for i, test := range tests {
		if test.valid != isPeriod(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsHostname(t *testing.T) {
	tests := []test{
		{"www.example.com", true},
		{strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 61), true},
		{strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 61) + ".", true},
		{strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 62) + ".", false}, // length more than 253 characters long
		{"www..com", false}, // empty label
		{"-a-host-name-that-starts-with--", false},
		{"not_a_valid_host_name", false},
		{"a-vvvvvvvvvvvvvvvveeeeeeeeeeeeeeeerrrrrrrrrrrrrrrryyyyyyyyyyyyyyyy-long-host-name-component", false},
		{"www.example-.com", false}, // label ends with a hyphen
	}
	for i, test := range tests {
		if test.valid != isHostname(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsEmail(t *testing.T) {
	tests := []test{
		{"joe.bloggs@example.com", true},
		{"2962", false},                                   // no "@" character
		{strings.Repeat("a", 244) + "@google.com", false}, // more than 254 characters long
		{strings.Repeat("a", 65) + "@google.com", false},  // local part more than 64 characters long
		{"santhosh@-google.com", false},                   // invalid domain name
	}
	for i, test := range tests {
		if test.valid != isEmail(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsIPV4(t *testing.T) {
	tests := []test{
		{"192.168.0.1", true},
		{"192.168.0.test", false},  // non-integer component
		{"127.0.0.0.1", false},     // too many components
		{"256.256.256.256", false}, // out-of-range values
		{"127.0", false},           // without 4 components
		{"0x7f000001", false},      // an integer
	}
	for i, test := range tests {
		if test.valid != isIPV4(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsIPV6(t *testing.T) {
	tests := []test{
		{"::1", true},
		{"192.168.0.1", false},                     // is IPV4
		{"12345::", false},                         // out-of-range values
		{"1:1:1:1:1:1:1:1:1:1:1:1:1:1:1:1", false}, // too many components
		{"::laptop", false},                        // containing illegal characters
	}
	for i, test := range tests {
		if test.valid != isIPV6(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsURI(t *testing.T) {
	tests := []test{
		{"http://foo.bar/?baz=qux#quux", true},
		{"//foo.bar/?baz=qux#quux", false}, // an invalid protocol-relative URI Reference
		{"\\\\WINDOWS\\fileshare", false},  // an invalid URI
		{"abc", false},                     // an invalid URI though valid URI reference
	}
	for i, test := range tests {
		if test.valid != isURI(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsURITemplate(t *testing.T) {
	tests := []test{
		{"http://example.com/dictionary/{term:1}/{term}", true},
		{"http://example.com/dictionary/{term:1}/{term", false},
		{"http://example.com/dictionary", true}, // without variables
		{"dictionary/{term:1}/{term}", true},    // relative url-template
	}
	for i, test := range tests {
		if test.valid != isURITemplate(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsRegex(t *testing.T) {
	tests := []test{
		{"([abc])+\\s+$", true},
		{"^(abc]", false}, // unclosed parenthesis
	}
	for i, test := range tests {
		if test.valid != isRegex(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsJSONPointer(t *testing.T) {
	tests := []test{
		{"/foo/bar~0/baz~1/%a", true},
		{"/foo/baz~", false}, // ~ not escaped
		{"/foo//bar", true},  // empty segment
		{"/foo/bar/", true},  // last empty segment
		{"", true},           // empty
		{"/foo", true},
		{"/foo/0", true},
		{"/ ", true},
		{"/a~1b", true},
		{"/c%d", true},
		{"/e^f", true},
		{"/g|h", true},
		{"/i\\j", true},
		{"/k\"l", true},
		{"/ ", true},
		{"/m~0n", true},
		{"/foo/-", true},      // used adding to the last array position
		{"/foo/-/bar", true},  // - used as object member name
		{"/~1~0~0~1~1", true}, // multiple escaped characters
		// escaped with fraction part
		{"/~1.1", true},
		{"/~0.1", true},
		// URI Fragment Identifier
		{"#", false},
		{"#/", false},
		{"#a", false},
		// some escaped, but not all
		{"/~0~", false},
		{"/~0/~", false},
		// wrong escape character
		{"/~2", false},
		{"/~-1", false},
		{"/~~", false}, // multiple characters not escaped
		// isn't empty nor starts with /
		{"a", false},
		{"0", false},
		{"a/a", false},
	}
	for i, test := range tests {
		if test.valid != isJSONPointer(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestRelativeJSONPointer(t *testing.T) {
	tests := []test{
		{"1", true},             // upwards RJP
		{"0/foo/bar", true},     // downwards RJP
		{"2/0/baz/1/zip", true}, // up and then down RJP, with array index
		{"0#", true},            // taking the member or index name
		{"/foo/bar", false},     // valid json-pointer, but invalid RJP
		{"-1/foo/bar", false},   // negative prefix
		{"0##", false},          // ## is not a valid json-pointer
		{"01/a", false},         // zero cannot be followed by other digits, plus json-pointer
		{"01#", false},          // zero cannot be followed by other digits, plus octothorpe
		{"", false},             // empty string
	}
	for i, test := range tests {
		if test.valid != isRelativeJSONPointer(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}

func TestIsUUID(t *testing.T) {
	tests := []test{
		{"2EB8AA08-AA98-11EA-B4AA-73B441D16380", true},   // valid: all upper-case
		{"2eb8aa08-aa98-11ea-b4aa-73b441d16380", true},   // valid: all lower-case
		{"2eb8aa08-AA98-11ea-B4Aa-73B441D16380", true},   // valid: mixed case
		{"00000000-0000-0000-0000-000000000000", true},   // valid: all zeroes
		{"2eb8aa08-aa98-11ea-b4aa-73b441d1638", false},   // invalid: wrong length
		{"2eb8aa08-aa98-11ea-73b441d16380", false},       // invalid: missing section
		{"2eb8aa08-aa98-11ea-b4ga-73b441d16380", false},  // invalid: bad characters (not hex)
		{"2eb8aa08aa9811eab4aa73b441d16380", false},      // invalid: no dashes
		{"2eb8aa08aa98-11ea-b4aa73b441d16380", false},    // too few dashes
		{"2eb8-aa08-aa98-11ea-b4aa73b44-1d16380", false}, // too many dashes
		{"2eb8aa08aa9811eab4aa73b441d16380----", false},  // dashes in the wrong spot
		{"98d80576-482e-427f-8434-7f86890ab222", true},   // valid: version 4
		{"99c17cbb-656f-564a-940f-1a4568f03487", true},   // valid: version 5
		{"99c17cbb-656f-664a-940f-1a4568f03487", true},   // valid: hypothetical version 6
		{"99c17cbb-656f-f64a-940f-1a4568f03487", true},   // valid: hypothetical version 15
	}
	for i, test := range tests {
		if test.valid != isUUID(test.str) {
			t.Errorf("#%d: %q, valid %t, got valid %t", i, test.str, test.valid, !test.valid)
		}
	}
}
