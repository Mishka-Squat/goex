package colorex

import (
	"errors"
	"image/color"

	"github.com/Mishka-Squat/goex/gx"
	"go.yaml.in/yaml/v4"
)

var errInvalidFormat = errors.New("invalid format")

type RGBA color.RGBA

func (c RGBA) IsZero() bool {
	return c.R == 0 &&
		c.G == 0 &&
		c.B == 0 &&
		c.A == 0
}

func (c *RGBA) UnmarshalYAML(value *yaml.Node) error {
	var str string
	if err := value.Decode(&str); err != nil {
		return err
	}

	err := ParseHexColorTo(value.Value, (*color.RGBA)(c))
	if err != nil {
		return err
	}

	return nil
}

func (c RGBA) ParseToAny(s string) any {
	return RGBA(gx.Must(ParseHexColor(s)))
}

func ParseHexColor(s string) (c color.RGBA, err error) {
	err = ParseHexColorTo(s, &c)
	return
}

func ParseHexColorTo(s string, c *color.RGBA) (err error) {
	c.A = 0xff

	if s[0] != '#' {
		return errInvalidFormat
	}

	hexToByte := func(b byte) byte {
		switch {
		case b >= '0' && b <= '9':
			return b - '0'
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10
		}
		err = errInvalidFormat
		return 0
	}

	switch len(s) {
	case 9:
		c.R = hexToByte(s[1])<<4 + hexToByte(s[2])
		c.G = hexToByte(s[3])<<4 + hexToByte(s[4])
		c.B = hexToByte(s[5])<<4 + hexToByte(s[6])
		c.A = hexToByte(s[7])<<4 + hexToByte(s[8])
	case 7:
		c.R = hexToByte(s[1])<<4 + hexToByte(s[2])
		c.G = hexToByte(s[3])<<4 + hexToByte(s[4])
		c.B = hexToByte(s[5])<<4 + hexToByte(s[6])
	case 4:
		c.R = hexToByte(s[1]) * 17
		c.G = hexToByte(s[2]) * 17
		c.B = hexToByte(s[3]) * 17
	default:
		return errInvalidFormat
	}

	return
}
