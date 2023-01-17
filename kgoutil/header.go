package kgoutil

import (
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/propagation"
)

/**  包装kgo.record，适配text carrier接口
  *  @author tryao
  *  @date 2022/12/16 08:52
**/

type Carrier struct {
	*kgo.Record
}

func AsCarrier(r *kgo.Record) propagation.TextMapCarrier {
	return &Carrier{r}
}

func (c *Carrier) Get(key string) string {
	for _, header := range c.Headers {
		if header.Key == key {
			return string(header.Value)
		}
	}
	return ""
}

func (c *Carrier) Set(key string, value string) {
	c.Headers = append(c.Headers, kgo.RecordHeader{
		Key:   key,
		Value: []byte(value),
	})
}

func (c *Carrier) Keys() []string {
	r := make([]string, 0)
	for _, header := range c.Headers {
		r = append(r, header.Key)
	}
	return r
}
