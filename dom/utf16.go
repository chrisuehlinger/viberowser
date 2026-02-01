package dom

// UTF16Length returns the length of a string in UTF-16 code units.
// This is used for DOM Range operations which work with character offsets
// as JavaScript defines them (UTF-16 code units, not bytes or grapheme clusters).
func UTF16Length(s string) int {
	return len(stringToUTF16(s))
}

// UTF16OffsetToByteOffset converts a UTF-16 code unit offset to a byte offset.
// Returns -1 if the offset is out of bounds.
func UTF16OffsetToByteOffset(s string, utf16Offset int) int {
	if utf16Offset < 0 {
		return -1
	}

	data := []byte(s)
	utf16Count := 0
	byteOffset := 0

	for byteOffset < len(data) {
		if utf16Count >= utf16Offset {
			return byteOffset
		}

		// Decode one rune
		c := data[byteOffset]
		var size int
		var r rune

		if c < 0x80 {
			r = rune(c)
			size = 1
		} else if c < 0xC0 {
			// Invalid continuation byte
			r = 0xFFFD
			size = 1
		} else if c < 0xE0 && byteOffset+2 <= len(data) {
			r = rune(c&0x1F)<<6 | rune(data[byteOffset+1]&0x3F)
			size = 2
		} else if c < 0xF0 && byteOffset+3 <= len(data) {
			r = rune(c&0x0F)<<12 | rune(data[byteOffset+1]&0x3F)<<6 | rune(data[byteOffset+2]&0x3F)
			size = 3
		} else if c < 0xF8 && byteOffset+4 <= len(data) {
			r = rune(c&0x07)<<18 | rune(data[byteOffset+1]&0x3F)<<12 | rune(data[byteOffset+2]&0x3F)<<6 | rune(data[byteOffset+3]&0x3F)
			size = 4
		} else {
			r = 0xFFFD
			size = 1
		}

		// Count UTF-16 code units for this rune
		if r >= 0x10000 {
			// Supplementary plane character uses 2 UTF-16 code units (surrogate pair)
			utf16Count += 2
		} else {
			utf16Count++
		}

		byteOffset += size
	}

	// Handle offset at end of string
	if utf16Count == utf16Offset {
		return byteOffset
	}

	return -1
}

// ByteOffsetToUTF16Offset converts a byte offset to a UTF-16 code unit offset.
// Returns -1 if the byte offset is out of bounds.
func ByteOffsetToUTF16Offset(s string, byteOffset int) int {
	if byteOffset < 0 || byteOffset > len(s) {
		return -1
	}

	data := []byte(s)
	utf16Count := 0
	i := 0

	for i < len(data) && i < byteOffset {
		c := data[i]
		var size int
		var r rune

		if c < 0x80 {
			r = rune(c)
			size = 1
		} else if c < 0xC0 {
			r = 0xFFFD
			size = 1
		} else if c < 0xE0 && i+2 <= len(data) {
			r = rune(c&0x1F)<<6 | rune(data[i+1]&0x3F)
			size = 2
		} else if c < 0xF0 && i+3 <= len(data) {
			r = rune(c&0x0F)<<12 | rune(data[i+1]&0x3F)<<6 | rune(data[i+2]&0x3F)
			size = 3
		} else if c < 0xF8 && i+4 <= len(data) {
			r = rune(c&0x07)<<18 | rune(data[i+1]&0x3F)<<12 | rune(data[i+2]&0x3F)<<6 | rune(data[i+3]&0x3F)
			size = 4
		} else {
			r = 0xFFFD
			size = 1
		}

		if r >= 0x10000 {
			utf16Count += 2
		} else {
			utf16Count++
		}

		i += size
	}

	return utf16Count
}

// UTF16Substring extracts a substring using UTF-16 code unit offsets.
// This properly handles multi-byte UTF-8 characters by converting offsets.
func UTF16Substring(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		return ""
	}

	startByte := UTF16OffsetToByteOffset(s, start)
	if startByte < 0 {
		return ""
	}

	endByte := UTF16OffsetToByteOffset(s, end)
	if endByte < 0 {
		endByte = len(s)
	}

	return s[startByte:endByte]
}

// UTF16SliceFrom returns the substring from a UTF-16 offset to the end.
func UTF16SliceFrom(s string, start int) string {
	startByte := UTF16OffsetToByteOffset(s, start)
	if startByte < 0 {
		return ""
	}
	return s[startByte:]
}

// UTF16SliceTo returns the substring from the beginning to a UTF-16 offset.
func UTF16SliceTo(s string, end int) string {
	if end <= 0 {
		return ""
	}
	endByte := UTF16OffsetToByteOffset(s, end)
	if endByte < 0 {
		return s
	}
	return s[:endByte]
}

// stringToUTF16 converts a Go string to UTF-16 code units.
func stringToUTF16(s string) []uint16 {
	result := make([]uint16, 0, len(s))
	data := []byte(s)

	for i := 0; i < len(data); {
		// Check for WTF-8 encoded surrogate (ED A0-BF 80-BF)
		if i+2 < len(data) && data[i] == 0xED {
			b1, b2 := data[i+1], data[i+2]
			if (b1 >= 0xA0 && b1 <= 0xBF) && (b2 >= 0x80 && b2 <= 0xBF) {
				// Decode WTF-8 surrogate
				cu := uint16((uint32(0xED&0x0F) << 12) | (uint32(b1&0x3F) << 6) | uint32(b2&0x3F))
				result = append(result, cu)
				i += 3
				continue
			}
		}

		// Decode as normal UTF-8
		c := data[i]
		var r rune
		var size int

		if c < 0x80 {
			r = rune(c)
			size = 1
		} else if c < 0xC0 {
			r = 0xFFFD
			size = 1
		} else if c < 0xE0 && i+2 <= len(data) {
			r = rune(c&0x1F)<<6 | rune(data[i+1]&0x3F)
			size = 2
		} else if c < 0xF0 && i+3 <= len(data) {
			r = rune(c&0x0F)<<12 | rune(data[i+1]&0x3F)<<6 | rune(data[i+2]&0x3F)
			size = 3
		} else if c < 0xF8 && i+4 <= len(data) {
			r = rune(c&0x07)<<18 | rune(data[i+1]&0x3F)<<12 | rune(data[i+2]&0x3F)<<6 | rune(data[i+3]&0x3F)
			size = 4
		} else {
			r = 0xFFFD
			size = 1
		}

		if r >= 0x10000 {
			// Supplementary plane character - encode as surrogate pair
			r -= 0x10000
			result = append(result, uint16(0xD800+(r>>10)))
			result = append(result, uint16(0xDC00+(r&0x3FF)))
		} else {
			result = append(result, uint16(r))
		}

		i += size
	}
	return result
}
