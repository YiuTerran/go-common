package nacos

import (
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/YiuTerran/go-common/base/log"
	"sort"
)

// ChooseBest 用于手动负载均衡，根据服务名选择最佳实例
func ChooseBest(serviceName string, fn func(i, j *model.Instance) bool) *model.Instance {
	all, err := GetNamingClient().SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		GroupName:   instanceParam.GroupName,
		HealthyOnly: true,
	})
	if err != nil || len(all) == 0 {
		log.Error("no healthy %s instance!!!", serviceName)
		return nil
	}
	sort.Slice(all, func(i, j int) bool {
		return fn(&all[i], &all[j])
	})
	return &all[0]
}
