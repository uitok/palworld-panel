package appconfig

import "encoding/hex"

const bundledSteamKeyMask byte = 0x55

var bundledSteamKeyBytes = [...]byte{
	0x2e, 0x9d, 0x64, 0xd1, 0x81, 0x68, 0x7d, 0xad,
	0x88, 0xff, 0x47, 0xa5, 0x10, 0x08, 0x17, 0x81,
}

func bundledSteamWebAPIKey() string {
	decoded := make([]byte, len(bundledSteamKeyBytes))
	for index, value := range bundledSteamKeyBytes {
		decoded[index] = value ^ bundledSteamKeyMask
	}
	return hex.EncodeToString(decoded)
}
