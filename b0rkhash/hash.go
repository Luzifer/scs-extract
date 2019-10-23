package b0rkhash

import (
	"encoding/binary"
)

// Some primes between 2^63 and 2^64 for various uses.
const (
	k0 uint64 = 0xc3a5c85c97cb3127
	k1 uint64 = 0xb492b66fbe98f273
	k2 uint64 = 0x9ae16a3b2f90404f
	k3 uint64 = 0xc949d7c7509e6557
)

func fetch32(p []byte) uint32 {
	return binary.LittleEndian.Uint32(p)
}

func fetch64(p []byte) uint64 {
	r := binary.LittleEndian.Uint64(p)
	return r
}

// Bitwise right rotate
func rotate(val uint64, shift uint) uint64 {
	// Avoid shifting by 64: doing so yields an undefined result.
	if shift == 0 {
		return val
	}
	return (val >> shift) | val<<(64-shift)
}

func rotateByAtleast1(val uint64, shift uint) uint64 {
	return (val >> shift) | (val << (64 - shift))
}

func shiftMix(val uint64) uint64 {
	return val ^ (val >> 47)
}

func hash128to64(x Uint128) uint64 {
	const mul = uint64(0x9ddfea08eb382d69)
	a := (x.Low64() ^ x.High64()) * mul
	a ^= (a >> 47)
	b := (x.High64() ^ a) * mul
	b ^= (b >> 47)
	b *= mul
	return b
}

func hashLen16(u, v uint64) uint64 {
	return hash128to64(Uint128{u, v})
}

func hashLen0to16(s []byte, length int) uint64 {
	if length > 8 {
		a := fetch64(s)
		b := fetch64(s[length-8:])
		return hashLen16(a, rotateByAtleast1(b+uint64(length), uint(length))) ^ b
	}

	if length >= 4 {
		a := uint64(fetch32(s))
		return hashLen16(uint64(length)+(a<<3), uint64(fetch32(s[length-4:])))
	}

	if length > 0 {
		a := uint8(s[0])
		b := uint8(s[length>>1])
		c := uint8(s[length-1])
		y := uint32(a) + (uint32(b) << 8)
		z := uint32(length) + (uint32(c) << 2)
		return shiftMix(uint64(y)*k2^uint64(z)*k3) * k2
	}

	return k2
}

// This probably works well for 16-byte strings as well, but is may be overkill
// in that case
func hashLen17to32(s []byte, length int) uint64 {
	a := fetch64(s) * k1
	b := fetch64(s[8:])
	c := fetch64(s[length-8:]) * k2
	d := fetch64(s[length-16:]) * k0
	return hashLen16(rotate(a-b, 43)+rotate(c, 30)+d,
		a+rotate(b^k3, 20)-c+uint64(length))
}

// Return a 16-byte hash for 48 bytes. Quick and dirty.
// callers do best to use "random-looking" values for a and b.
func weakHashLen32WithSeeds(w, x, y, z, a, b uint64) Uint128 {
	a += w
	b = rotate(b+a+z, 21)
	c := a
	a += x
	a += y
	b += rotate(a, 44)
	return Uint128{a + z, b + c}
}

// Return a 16-byte hash for s[0] ... s[31], a, and b. Quick and dirty.
func weakHashLen32WithSeedsByte(s []byte, a, b uint64) Uint128 {
	return weakHashLen32WithSeeds(
		fetch64(s),
		fetch64(s[8:]),
		fetch64(s[16:]),
		fetch64(s[24:]),
		a,
		b)
}

// Return an 8-byte hash for 33 to 64 bytes.
func hashLen33to64(s []byte, length int) uint64 {
	z := fetch64(s[24:])
	a := fetch64(s) + (uint64(length)+fetch64(s[length-16:]))*k0
	b := rotate(a+z, 52)
	c := rotate(a, 37)
	a += fetch64(s[8:])
	c += rotate(a, 7)
	a += fetch64(s[16:])
	vf := a + z
	vs := b + rotate(a, 31) + c
	a = fetch64(s[16:]) + fetch64(s[length-32:])
	z = fetch64(s[length-8:])
	b = rotate(a+z, 52)
	c = rotate(a, 37)
	a += fetch64(s[length-24:])
	c += rotate(a, 7)
	a += fetch64(s[length-16:])
	wf := a + z
	ws := b + rotate(a, 31) + c
	r := shiftMix((vf+ws)*k2 + (wf+vs)*k0)
	return shiftMix(r*k0+vs) * k2
}

// CityHash64 return a 64-bit hash.
func CityHash64(s []byte) uint64 {
	length := len(s)
	if length <= 32 {
		if length <= 16 {
			return hashLen0to16(s, length)
		}
		return hashLen17to32(s, length)
	} else if length <= 64 {
		return hashLen33to64(s, length)
	}

	// For string over 64 bytes we hash the end first, and then as we
	// loop we keep 56 bytes of state: v, w, x, y and z.
	x := fetch64(s[length-40:])
	y := fetch64(s[length-16:]) + fetch64(s[length-56:])
	z := hashLen16(fetch64(s[length-48:])+uint64(length), fetch64(s[length-24:]))
	v := weakHashLen32WithSeedsByte(s[length-64:], uint64(length), z)
	w := weakHashLen32WithSeedsByte(s[length-32:], y+k1, x)
	x = x*k1 + fetch64(s)

	// Decrease len to the nearest multiple of 64, and operate on 64-byte chunks.
	tmpLength := uint32(length)
	tmpLength = uint32(tmpLength-1) & ^uint32(63)
	for {
		x = rotate(x+y+v.Low64()+fetch64(s[8:]), 37) * k1
		y = rotate(y+v.High64()+fetch64(s[48:]), 42) * k1
		x ^= w.High64()
		y += v.Low64() + fetch64(s[40:])
		z = rotate(z+w.Low64(), 33) * k1
		v = weakHashLen32WithSeedsByte(s, v.High64()*k1, x+w.Low64())
		w = weakHashLen32WithSeedsByte(s[32:], z+w.High64(), y+fetch64(s[16:]))
		z, x = x, z
		s = s[64:]
		tmpLength -= 64
		if tmpLength == 0 {
			break
		}
	}

	return hashLen16(
		hashLen16(v.Low64(), w.Low64())+shiftMix(y)*k1+z,
		hashLen16(v.High64(), w.High64())+x)
}

// CityHash64WithSeed return a 64-bit hash with a seed.
func CityHash64WithSeed(s []byte, seed uint64) uint64 {
	return CityHash64WithSeeds(s, k2, seed)
}

// CityHash64WithSeeds return a 64-bit hash with two seeds.
func CityHash64WithSeeds(s []byte, seed0, seed1 uint64) uint64 {
	return hashLen16(CityHash64(s)-seed0, seed1)
}
