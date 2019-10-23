package b0rkhash

import "testing"

func TestCityHash64(t *testing.T) {
	for input, expect := range map[string]uint64{
		``:                                  0x9AE16A3B2F90404F,
		`custom/city.sii`:                   0x1ffe051698fba3e2,
		`def`:                               0x2C6F469EFB31C45A,
		`def/camera/city_start/actions.sii`: 0xa74e0b70addb8e2d,
		`def/city`:                          0x5e1b1d2c928270d1, // This is a definitive bug but also exists in SCS implementation
		`def/economy_data.sii`:              0xce3123f8a189862e,
		`def/map_data.sii`:                  0x73aded9d5c6b4762,
		`def/bank_data.sii`:                 0xdb6507b90c06f96a,
	} {
		h := CityHash64([]byte(input))
		if h != expect {
			t.Errorf("Unexpected hash for input %q: expect=0x%x result=0x%x", input, expect, h)
		} else {
			t.Logf("Success for input %q: expect=0x%x result=0x%x", input, expect, h)
		}
	}
}
