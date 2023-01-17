package nacos

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/spf13/viper"
	"github.com/YiuTerran/go-common/base/log"
	"github.com/YiuTerran/go-common/base/util/netutil"
)

/**
  *  @author tryao
  *  @date 2022/04/07 08:55
**/
var (
	once          sync.Once
	defaultGroup  string
	defaultDataId string
	clientConfig  constant.ClientConfig // 客户端配置
	instanceParam vo.RegisterInstanceParam
	env           string

	configClient config_client.IConfigClient
	namingClient naming_client.INamingClient
)

// GetViper 通过group和dataId生成viper
// 注意这里没有没有用once，所以每次调用都会产生一个新的
// 配置变化时，回调函数默认会处理日志等级，如果有其他回调也可以在调用的时候传入
// 注意这里所有配置文件必须在同一个namespace
func GetViper(group, dataId string, cbs ...func(*viper.Viper)) (*SafeViper, chan struct{}) {
	suffix := "yaml"
	idx := strings.LastIndex(dataId, ".")
	if idx >= 0 {
		suffix = dataId[idx+1:]
	}
	sv := &SafeViper{}
	fn := func(namespace, group, dataId, data string) {
		vp := viper.New()
		vp.SetConfigType(suffix)
		if err := vp.ReadConfig(bytes.NewReader([]byte(data))); err != nil {
			log.Error("fail parse config file, group: %s, dataId: %s", group, dataId)
		} else {
			for _, cb := range cbs {
				cb(vp)
			}
			//仅当解析成功才替换掉
			sv.Store(vp)
		}
	}
	// 第一次读入文件
	data, err := configClient.GetConfig(vo.ConfigParam{
		DataId: dataId,
		Group:  group,
	})
	if err != nil || data == "" {
		log.Fatal("fail to read config from %s: data:%s, err:%v", dataId, data, err)
	}
	fn("", group, dataId, data)
	if sv.Load() == nil {
		log.Fatal("fail to parse config file:%s, err:%s", dataId, err)
	}
	// 监听文件变化
	err = configClient.ListenConfig(vo.ConfigParam{
		DataId:   dataId,
		Group:    group,
		OnChange: fn,
	})
	if err != nil {
		log.Fatal("fail to comm with nacos:%s", err)
	}
	ch := make(chan struct{}, 1)
	go func() {
		<-ch
		_ = configClient.CancelListenConfig(vo.ConfigParam{
			DataId: dataId,
			Group:  group,
		})
	}()
	return sv, ch
}

// GetDefaultViper 获取默认的配置，这里直接做了日志等级自动切换
func GetDefaultViper(cbs ...func(*viper.Viper)) (*SafeViper, chan struct{}) {
	watchLogLevel := func(vp *viper.Viper) {
		vp.SetDefault("log.level", "debug")
		nl := strings.ToLower(vp.GetString("log.level"))
		log.ChangeLogLevel(log.Level(nl))
	}
	cbs = append(cbs, watchLogLevel)
	sv, ch := GetViper(defaultGroup, defaultDataId, cbs...)
	vp := sv.Load()
	//初始化日志配置，path需要环境变量展开适配k8s环境
	path := os.ExpandEnv(vp.GetString("log.path"))
	tp := log.OutType(vp.GetInt("log.type"))
	if tp == 0 {
		tp = log.OutTypeAlias(vp.GetString("log.type"))
	}
	log.Builder.
		Name(vp.GetString("log.name")).
		Path(path).
		EnableRotate(vp.GetBool("log.rotate")).
		OutType(tp).
		MaxAge(vp.GetInt("log.max-age")).
		MaxSize(vp.GetInt("log.max-size")).
		MaxBackUps(vp.GetInt("log.max-backup")).
		Build()
	watchLogLevel(vp)
	return sv, ch
}

// GetConfigClient 获取配置客户端，默认配置在应用程序同目录或者_config文件夹下
// 默认配置文件的格式见bootstrap.yml，和java的配置保持一致方便运维人员操作
// 通过注册回调函数完成配置变更监听
func GetConfigClient() config_client.IConfigClient {
	return configClient
}

// GetNamingClient 获取负载均衡器示例
// 会自动将当前节点注册到NACOS，逻辑同java，可以通过namingClient获取其他微服务实例
// 从而实现客户端负载均衡
func GetNamingClient() naming_client.INamingClient {
	return namingClient
}

func InstanceParam() vo.RegisterInstanceParam {
	return instanceParam
}

// Close 服务关闭时注销client
func Close() {
	_, _ = namingClient.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          instanceParam.Ip,
		Port:        instanceParam.Port,
		Cluster:     instanceParam.ClusterName,
		ServiceName: instanceParam.ServiceName,
		GroupName:   instanceParam.GroupName,
		Ephemeral:   instanceParam.Ephemeral,
	})
}

func initConfigClient(vp *viper.Viper) []constant.ServerConfig {
	app := vp.GetString("application.name")
	if app == "" {
		log.Fatal("必须设置application.name")
	}
	defaultGroup = vp.GetString("nacos.instance.groupName")
	if defaultGroup == "" {
		defaultGroup = "DEFAULT_GROUP"
	}
	suffix := vp.GetString("application.confFormat")
	if suffix == "" {
		suffix = "yaml"
	}
	if env != "" {
		defaultDataId = fmt.Sprintf("%s-%s.yaml", app, env)
	} else {
		defaultDataId = app + "." + suffix
	}
	log.Info("nacos default config group:%s, dataId:%s", defaultGroup, defaultDataId)
	// 服务端配置
	var serverConfig []constant.ServerConfig
	err := vp.UnmarshalKey("nacos.server", &serverConfig)
	if err != nil {
		log.Fatal("fail to read nacos server conf")
	}
	// 客户端配置
	err = vp.UnmarshalKey("nacos.client", &clientConfig)
	if err != nil {
		log.Fatal("fail to read nacos client conf")
	}
	if clientConfig.NamespaceId == "public" {
		//public必须配置成空的
		clientConfig.NamespaceId = ""
	}
	//初始化日志配置，写死，现在没办法让他不打日志
	clientConfig.LogDir = "./logs"
	clientConfig.LogLevel = "error"
	clientConfig.CustomLogger = &BaseLogger{}
	configClient, err = clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  &clientConfig,
		ServerConfigs: serverConfig,
	})
	if err != nil {
		log.Fatal("fail to create configClient:%w", err)
	}
	return serverConfig
}

func initNamingClient(vp *viper.Viper, serverConfig []constant.ServerConfig, autoRegister bool) {
	// 生成唯一实例
	var err error
	namingClient, err = clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  &clientConfig,
		ServerConfigs: serverConfig,
	})
	if err != nil {
		log.Fatal("fail to create namingClient:%w", err)
	}
	//注册自己
	if vp.Get("nacos.instance") != nil {
		if err = vp.UnmarshalKey("nacos.instance", &instanceParam); err != nil {
			log.Fatal("fail to parse nacos.instance")
		}
	}
	if instanceParam.Ip != "" {
		if net.ParseIP(instanceParam.Ip) == nil {
			if ip, err := netutil.FilterSelfIP(instanceParam.Ip); err != nil {
				log.Fatal("fail to register current instance:%w", err)
			} else {
				instanceParam.Ip = ip.String()
			}
		}
	} else {
		if tmp, err := netutil.FilterSelfIP(""); err != nil {
			log.Fatal("fail to register current instance:%w", err)
		} else {
			instanceParam.Ip = tmp.String()
		}
	}
	log.Info("nacos register service instance ip:%s", instanceParam.Ip)
	instanceParam.Enable = true
	instanceParam.Healthy = true
	if instanceParam.ServiceName == "" {
		instanceParam.ServiceName = vp.GetString("application.name")
	}
	if vp.Get("nacos.instanceParam.ephemeral") == nil {
		instanceParam.Ephemeral = true
	}
	if instanceParam.Port == 0 {
		instanceParam.Port = 8080
	}
	if instanceParam.Weight == 0 {
		instanceParam.Weight = 1
	}

	if autoRegister {
		if _, err = namingClient.RegisterInstance(instanceParam); err != nil {
			log.Fatal("fail to register current instance:%w", err)
		}
	}
}

// Init 初始化失败后会直接panic
func Init(configPath ...string) {
	InitBy(true, configPath...)
}

// RegisterInstance 手动注册实例
func RegisterInstance() error {
	if _, err := namingClient.RegisterInstance(instanceParam); err != nil {
		log.Fatal("fail to register current instance:%w", err)
		return err
	}

	log.Info("register instance:%+v success", instanceParam)
	return nil
}

// RegisterInstanceWithMeta 有参数的注册实例
func RegisterInstanceWithMeta(meta map[string]string) {
	instanceParam.Metadata = meta
	_ = RegisterInstance()
}

// RegisterInstanceWithAddr 手动注册服务地址信息 如外网地址手动注册
func RegisterInstanceWithAddr(ip string, port uint64) error {
	instanceParam.Ip = ip
	instanceParam.Port = port

	return RegisterInstance()
}

func RegisterInstanceWithAddrAndMeta(ip string, port uint64, meta map[string]string) error {
	instanceParam.Ip = ip
	instanceParam.Port = port
	instanceParam.Metadata = meta

	return RegisterInstance()
}

func InitBy(autoRegister bool, configPath ...string) {
	once.Do(func() {
		// 查找bootstrap.yml，初始化nacos相关配置
		vp := viper.New()
		vp.AutomaticEnv()
		// 优先读环境变量
		env = vp.GetString("ENV")
		vp.AddConfigPath("./_config")
		vp.AddConfigPath("./configs")
		vp.AddConfigPath(".")
		// 先读入默认配置
		cf := "bootstrap"
		vp.SetConfigName(cf)
		for _, s := range configPath {
			vp.AddConfigPath(s)
		}
		var (
			defaultFileError error
			envFileError     = fmt.Errorf("no env file")
		)
		defaultFileError = vp.ReadInConfig()
		// 再尝试读入环境特定配置
		if env != "" {
			cf += "-" + env
			vp.SetConfigName(cf)
			envFileError = vp.MergeInConfig()
		}
		if defaultFileError != nil && envFileError != nil {
			log.Fatal("fail to read bootstrap file")
		}
		if env == "" {
			//其次读配置文件
			env = vp.GetString("application.env")
		}
		serverConfig := initConfigClient(vp)
		initNamingClient(vp, serverConfig, autoRegister)
	})
}
