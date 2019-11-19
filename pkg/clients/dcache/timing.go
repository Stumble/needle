package dcache

import "time"

type CacheDuration time.Duration

const (
	SuperFast           CacheDuration = CacheDuration(time.Second)
	VeryFast            CacheDuration = CacheDuration(time.Second * 2)
	Fast                CacheDuration = CacheDuration(time.Second * 5)
	Normal              CacheDuration = CacheDuration(time.Second * 10)
	Slow                CacheDuration = CacheDuration(time.Second * 30)
	VerySlow            CacheDuration = CacheDuration(time.Minute * 1)
	SuperSlow           CacheDuration = CacheDuration(time.Minute * 5)
	SuperDuperSlow      CacheDuration = CacheDuration(time.Hour * 1)
	SuperSuperDuperSlow CacheDuration = CacheDuration(time.Hour * 5)
	NeverExpire         CacheDuration = CacheDuration(0)
)

func (c CacheDuration) ToDuration() time.Duration {
	return time.Duration(c)
}
