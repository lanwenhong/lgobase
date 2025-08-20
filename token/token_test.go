package token

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/token"
)

func TestTokenPack(t *testing.T) {
	tk := &token.Token{
		Ver:    1,
		Idc:    2,
		FlagUC: "u",
		//Uid:      0xFFFFFFFFF,
		Uid:      1,
		OpUid:    10,
		Expire:   100000,
		Deadline: 0xFFFFFFFFF,
		Udid:     "0",
		Tkey:     "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
	}
	ctx := context.Background()
	tk.Pack(ctx)
}

func TestTokenUnPack(t *testing.T) {
	//bdata := "AW6NAAWV4ZpxruynLHNVV2iLKqIp5rTNGSXGBY51/dxlz85Gb7Mgx70s268="
	//bdata := "AVsRwJSB9I2GKcTBeuLL150bvavuYz9aGzJ5au+d4WAvxRAuZQ1P7IEviv8ed111OzynGDZqd9ByLBV6"
	//bdata := "AZaJQNaOxZYFNJUJCd3q9CYLdq/fUy9iIz/jprdpz35C0L18qWCZF2WUWoIiW4ervbEiactfhtmPg7wj"
	//bdata := "AQdJQHreidu9Ha2NANAEjoiQ5Zx6Do1VNY1IEfN96vCILdcDr9l0uNizrP4vQJOIZaCMvWmeSATifD9J"
	//bdata := "ASfgQIcyjzNROQyauMfIW3Kcxto/9UdoI6XKQBNc284NUvpUk3b6t6FsebFaW6X8+EYh+v3cx5WFouzL"
	//bdata := "AdCmQBX4FObetnGSbR/AizgTMwJ459Rt5qVCkKObUpdIfHnEHSldufJFErPRxLD1K5yth8ybXQ9jXHjg"
	//bdata := "AThpQClJQs2NlgX2iWBm6TNaC3OgKSQkvswOhconUaNDru8dfCdWB/MC0WfUYrWNKHvt77EDjTA+l/B8"
	//bdata := "AUFXQE5NCkwNCB/YIRyRJjpKYFdvUZtqN0J/jwj+nz5I4dlCSgP/82VohPc="
	//bdata := "AQ9wQKN0/3ARAOghH/do1hkJmhq8g5emCAlyOzHxqocBLgFD2Etm78mpq30="
	//bdata := "AXfIQKcRKN0O/DVZbbuYuxqpy4HKqrBmFFX+2FrFlZTDHkbokDJvS4FmBjw="
	//bdata := "AfhiQMOhM1rLsVE6x8UEUP0wWLUEZDcKNoEJj/fLdVuACxIJlPFsT0uJtKg="
	//bdata := "ARq6QEKJHR7RhrWP43dEA4lvPTmh/lkdWOmItrwlAywupATphLO5wfSPhnU="
	//bdata := "ATk5QHFB5KxGkMY/L98SJ7/AFBx+3/xfjeCqG5FsTJtTH55hV4VGpQ3/k3k="
	bdata := "ATk5QHFB5KxGkMY/FW724io7CYH4UTJE9PLRkW5zV8q6AzCXku/PIiz5ujY="
	ctx := context.Background()
	tk := &token.Token{
		Tkey: "IypMcRkPXkbeNDRl6Km43boHr98udp7o",
	}
	tk.UnPack(ctx, bdata)
}
