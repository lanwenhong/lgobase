package gconfig_v2_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/lanwenhong/lgobase/gconfig_v2"
)

func TestUnmarshalComplexGatewayConfig(t *testing.T) {
	content := []byte(`# 分布式网关完整业务配置
gateway_cluster:
  cluster_name: biz-main-gw
  version: 2.8.6
  enable: true
  max_conn: 65535
  idle_timeout: 30.5
  desc: '主业务网关集群，负责订单、支付、用户路由，禁止直接对外暴露'

  # 多节点实例数组，每个节点是独立映射
  node_list:
    - node_id: gw-01
      host: 10.0.1.10
      port: 8080
      weight: 100
      online: true
      tags: ["main", "order", "pay"]
      limit_rule:
        qps: 2000
        burst: 500
        black_ip: ["192.168.1.1", "10.99.0.100"]
        white_switch: false
      disk_info:
        root: 500.2G
        data: "2T"
        mount_list:
          - /data/logs
          - /data/storage/biz
          - /tmp/cache

    - node_id: gw-02
      host: 10.0.1.11
      port: 8080
      weight: 80
      online: true
      tags: ["backup", "user"]
      limit_rule:
        qps: 1200
        burst: 300
        black_ip: []
        white_switch: true
      disk_info:
        root: 500.2G
        data: "1.5T"
        mount_list:
          - /data/logs
          - /data/user/upload

    - node_id: gw-03
      host: 10.0.1.12
      port: 8081
      weight: 20
      online: false
      tags: ["test"]
      limit_rule:
        qps: 100
        burst: 20
        black_ip: ["172.16.0.5"]
        white_switch: false
      disk_info:
        root: 100G
        data: "500G"
        mount_list: []

  # 路由规则：数组嵌套多层映射
  route_rules:
    - rule_name: order_create
      path: /api/order/create
      method: ["POST"]
      target_service: order-service
      priority: 10
      header_filter:
        allow_header: ["token", "device-id", "version"]
        drop_header: ["internal-sign"]
      proxy_config:
        timeout: 2.8
        retry_times: 3
        retry_code: [500, 502, 503]
        upstream_pool:
          pool_name: order-pool
          instance:
            - ip: 10.0.2.20
              port: 9000
              status: online
            - ip: 10.0.2.21
              port: 9000
              status: online
            - ip: 10.0.2.22
              port: 9000
              status: offline

    - rule_name: user_info_query
      path: /api/user/info
      method: ["GET", "OPTIONS"]
      target_service: user-service
      priority: 5
      header_filter:
        allow_header: ["token", "uid"]
        drop_header: []
      proxy_config:
        timeout: 1.2
        retry_times: 1
        retry_code: [504]
        upstream_pool:
          pool_name: user-pool
          instance:
            - ip: 10.0.3.10
              port: 9100
              status: online

  # 跨模块权限配置
  permission:
    super_admin:
      uid_list: [10001, 10002, 10003]
      allow_api: ["*"]
      forbid_api: []
    operator:
      uid_list: [20001, 20002]
      allow_api:
        - /api/user/*
        - /api/order/query
      forbid_api:
        - /api/order/create
        - /api/pay/refund
    guest:
      uid_list: [90001]
      allow_api: [/api/public/info]
      forbid_api: ["*"]

# 监控告警独立大模块
monitor_alarm:
  enable: true
  collect_interval: 15
  metrics:
    basic: [cpu, mem, disk, network, conn]
    business: [qps, error_rate, latency_p99]
  threshold:
    cpu: 85.0
    mem: 90.0
    error_rate: 0.05
    latency_p99: 500
  notify_channel:
    dingtalk:
      enable: true
      webhook: "https://dingtalk.com/robot/send?token='abc123,xyz789'"
      receiver: ["dev-group", "ops-group"]
    sms:
      enable: false
      phone_list: ["13800138000", "13900139000"]
  history_retention:
    day: 30
    store_engine: influxdb
    conn_addr:
      addr: 10.0.4.50
      port: 8086
      db: gw_monitor
      auth:
        user: monitor_admin
        pass: 'admin''s@2026'

# 存储中间件配置
storage:
  redis_cluster:
    enable: true
    master_nodes:
      - 10.0.5.10:6379
      - 10.0.5.11:6379
    replica_nodes:
      - 10.0.5.12:6379
    password: "Redis@123456,Secure"
    db_index: 0
    pool_size: 200
    expire_map:
      token: 86400
      cache_goods: 3600
      hot_order: 600
  mysql:
    main:
      dsn: root:Mysql'Pass@tcp(10.0.6.30:3306)/biz_db
      max_open: 100
      max_idle: 20
      tables:
        - t_user
        - t_order
        - t_pay_log
    backup:
      dsn: root:123456@tcp(10.0.6.31:3306)/biz_db
      read_only: true
      slave_delay: 0.5
`)

	ctx := context.Background()

	var raw map[string]any
	if err := gconfig_v2.Unmarshal(ctx, content, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["gateway_cluster"].(map[string]any); !ok {
		t.Fatalf("raw gateway_cluster = %T", raw["gateway_cluster"])
	}

	type LimitRule struct {
		QPS         int      `yaml:"qps"`
		Burst       int      `yaml:"burst"`
		BlackIP     []string `yaml:"black_ip"`
		WhiteSwitch bool     `yaml:"white_switch"`
	}
	type DiskInfo struct {
		Root      string   `yaml:"root"`
		Data      string   `yaml:"data"`
		MountList []string `yaml:"mount_list"`
	}
	type Node struct {
		NodeID    string    `yaml:"node_id"`
		Host      string    `yaml:"host"`
		Port      int       `yaml:"port"`
		Weight    int       `yaml:"weight"`
		Online    bool      `yaml:"online"`
		Tags      []string  `yaml:"tags"`
		LimitRule LimitRule `yaml:"limit_rule"`
		DiskInfo  DiskInfo  `yaml:"disk_info"`
	}
	type HeaderFilter struct {
		AllowHeader []string `yaml:"allow_header"`
		DropHeader  []string `yaml:"drop_header"`
	}
	type UpstreamInstance struct {
		IP     string `yaml:"ip"`
		Port   int    `yaml:"port"`
		Status string `yaml:"status"`
	}
	type UpstreamPool struct {
		PoolName string             `yaml:"pool_name"`
		Instance []UpstreamInstance `yaml:"instance"`
	}
	type ProxyConfig struct {
		Timeout      float64      `yaml:"timeout"`
		RetryTimes   int          `yaml:"retry_times"`
		RetryCode    []int        `yaml:"retry_code"`
		UpstreamPool UpstreamPool `yaml:"upstream_pool"`
	}
	type RouteRule struct {
		RuleName      string       `yaml:"rule_name"`
		Path          string       `yaml:"path"`
		Method        []string     `yaml:"method"`
		TargetService string       `yaml:"target_service"`
		Priority      int          `yaml:"priority"`
		HeaderFilter  HeaderFilter `yaml:"header_filter"`
		ProxyConfig   ProxyConfig  `yaml:"proxy_config"`
	}
	type RolePermission struct {
		UIDList   []int    `yaml:"uid_list"`
		AllowAPI  []string `yaml:"allow_api"`
		ForbidAPI []string `yaml:"forbid_api"`
	}
	type Permission struct {
		SuperAdmin RolePermission `yaml:"super_admin"`
		Operator   RolePermission `yaml:"operator"`
		Guest      RolePermission `yaml:"guest"`
	}
	type GatewayCluster struct {
		ClusterName string      `yaml:"cluster_name"`
		Version     string      `yaml:"version"`
		Enable      bool        `yaml:"enable"`
		MaxConn     int         `yaml:"max_conn"`
		IdleTimeout float64     `yaml:"idle_timeout"`
		Desc        string      `yaml:"desc"`
		NodeList    []Node      `yaml:"node_list"`
		RouteRules  []RouteRule `yaml:"route_rules"`
		Permission  Permission  `yaml:"permission"`
	}
	type Metrics struct {
		Basic    []string `yaml:"basic"`
		Business []string `yaml:"business"`
	}
	type Threshold struct {
		CPU        float64 `yaml:"cpu"`
		Mem        float64 `yaml:"mem"`
		ErrorRate  float64 `yaml:"error_rate"`
		LatencyP99 int     `yaml:"latency_p99"`
	}
	type DingTalk struct {
		Enable   bool     `yaml:"enable"`
		Webhook  string   `yaml:"webhook"`
		Receiver []string `yaml:"receiver"`
	}
	type SMS struct {
		Enable    bool     `yaml:"enable"`
		PhoneList []string `yaml:"phone_list"`
	}
	type NotifyChannel struct {
		DingTalk DingTalk `yaml:"dingtalk"`
		SMS      SMS      `yaml:"sms"`
	}
	type Auth struct {
		User string `yaml:"user"`
		Pass string `yaml:"pass"`
	}
	type ConnAddr struct {
		Addr string `yaml:"addr"`
		Port int    `yaml:"port"`
		DB   string `yaml:"db"`
		Auth Auth   `yaml:"auth"`
	}
	type HistoryRetention struct {
		Day         int      `yaml:"day"`
		StoreEngine string   `yaml:"store_engine"`
		ConnAddr    ConnAddr `yaml:"conn_addr"`
	}
	type MonitorAlarm struct {
		Enable           bool             `yaml:"enable"`
		CollectInterval  int              `yaml:"collect_interval"`
		Metrics          Metrics          `yaml:"metrics"`
		Threshold        Threshold        `yaml:"threshold"`
		NotifyChannel    NotifyChannel    `yaml:"notify_channel"`
		HistoryRetention HistoryRetention `yaml:"history_retention"`
	}
	type RedisCluster struct {
		Enable       bool           `yaml:"enable"`
		MasterNodes  []string       `yaml:"master_nodes"`
		ReplicaNodes []string       `yaml:"replica_nodes"`
		Password     string         `yaml:"password"`
		DBIndex      int            `yaml:"db_index"`
		PoolSize     int            `yaml:"pool_size"`
		ExpireMap    map[string]int `yaml:"expire_map"`
	}
	type MySQLNode struct {
		DSN        string   `yaml:"dsn"`
		MaxOpen    int      `yaml:"max_open"`
		MaxIdle    int      `yaml:"max_idle"`
		Tables     []string `yaml:"tables"`
		ReadOnly   bool     `yaml:"read_only"`
		SlaveDelay float64  `yaml:"slave_delay"`
	}
	type MySQL struct {
		Main   MySQLNode `yaml:"main"`
		Backup MySQLNode `yaml:"backup"`
	}
	type Storage struct {
		RedisCluster RedisCluster `yaml:"redis_cluster"`
		MySQL        MySQL        `yaml:"mysql"`
	}
	type Config struct {
		GatewayCluster GatewayCluster `yaml:"gateway_cluster"`
		MonitorAlarm   MonitorAlarm   `yaml:"monitor_alarm"`
		Storage        Storage        `yaml:"storage"`
	}

	var cfg Config
	if err := gconfig_v2.Unmarshal(ctx, content, &cfg); err != nil {
		t.Fatal(err)
	}

	want := Config{
		GatewayCluster: GatewayCluster{
			ClusterName: "biz-main-gw",
			Version:     "2.8.6",
			Enable:      true,
			MaxConn:     65535,
			IdleTimeout: 30.5,
			Desc:        "主业务网关集群，负责订单、支付、用户路由，禁止直接对外暴露",
			NodeList: []Node{
				{
					NodeID: "gw-01",
					Host:   "10.0.1.10",
					Port:   8080,
					Weight: 100,
					Online: true,
					Tags:   []string{"main", "order", "pay"},
					LimitRule: LimitRule{
						QPS:         2000,
						Burst:       500,
						BlackIP:     []string{"192.168.1.1", "10.99.0.100"},
						WhiteSwitch: false,
					},
					DiskInfo: DiskInfo{
						Root:      "500.2G",
						Data:      "2T",
						MountList: []string{"/data/logs", "/data/storage/biz", "/tmp/cache"},
					},
				},
				{
					NodeID: "gw-02",
					Host:   "10.0.1.11",
					Port:   8080,
					Weight: 80,
					Online: true,
					Tags:   []string{"backup", "user"},
					LimitRule: LimitRule{
						QPS:         1200,
						Burst:       300,
						BlackIP:     []string{},
						WhiteSwitch: true,
					},
					DiskInfo: DiskInfo{
						Root:      "500.2G",
						Data:      "1.5T",
						MountList: []string{"/data/logs", "/data/user/upload"},
					},
				},
				{
					NodeID: "gw-03",
					Host:   "10.0.1.12",
					Port:   8081,
					Weight: 20,
					Online: false,
					Tags:   []string{"test"},
					LimitRule: LimitRule{
						QPS:         100,
						Burst:       20,
						BlackIP:     []string{"172.16.0.5"},
						WhiteSwitch: false,
					},
					DiskInfo: DiskInfo{
						Root:      "100G",
						Data:      "500G",
						MountList: []string{},
					},
				},
			},
			RouteRules: []RouteRule{
				{
					RuleName:      "order_create",
					Path:          "/api/order/create",
					Method:        []string{"POST"},
					TargetService: "order-service",
					Priority:      10,
					HeaderFilter: HeaderFilter{
						AllowHeader: []string{"token", "device-id", "version"},
						DropHeader:  []string{"internal-sign"},
					},
					ProxyConfig: ProxyConfig{
						Timeout:    2.8,
						RetryTimes: 3,
						RetryCode:  []int{500, 502, 503},
						UpstreamPool: UpstreamPool{
							PoolName: "order-pool",
							Instance: []UpstreamInstance{
								{IP: "10.0.2.20", Port: 9000, Status: "online"},
								{IP: "10.0.2.21", Port: 9000, Status: "online"},
								{IP: "10.0.2.22", Port: 9000, Status: "offline"},
							},
						},
					},
				},
				{
					RuleName:      "user_info_query",
					Path:          "/api/user/info",
					Method:        []string{"GET", "OPTIONS"},
					TargetService: "user-service",
					Priority:      5,
					HeaderFilter: HeaderFilter{
						AllowHeader: []string{"token", "uid"},
						DropHeader:  []string{},
					},
					ProxyConfig: ProxyConfig{
						Timeout:    1.2,
						RetryTimes: 1,
						RetryCode:  []int{504},
						UpstreamPool: UpstreamPool{
							PoolName: "user-pool",
							Instance: []UpstreamInstance{
								{IP: "10.0.3.10", Port: 9100, Status: "online"},
							},
						},
					},
				},
			},
			Permission: Permission{
				SuperAdmin: RolePermission{
					UIDList:   []int{10001, 10002, 10003},
					AllowAPI:  []string{"*"},
					ForbidAPI: []string{},
				},
				Operator: RolePermission{
					UIDList:   []int{20001, 20002},
					AllowAPI:  []string{"/api/user/*", "/api/order/query"},
					ForbidAPI: []string{"/api/order/create", "/api/pay/refund"},
				},
				Guest: RolePermission{
					UIDList:   []int{90001},
					AllowAPI:  []string{"/api/public/info"},
					ForbidAPI: []string{"*"},
				},
			},
		},
		MonitorAlarm: MonitorAlarm{
			Enable:          true,
			CollectInterval: 15,
			Metrics: Metrics{
				Basic:    []string{"cpu", "mem", "disk", "network", "conn"},
				Business: []string{"qps", "error_rate", "latency_p99"},
			},
			Threshold: Threshold{
				CPU:        85.0,
				Mem:        90.0,
				ErrorRate:  0.05,
				LatencyP99: 500,
			},
			NotifyChannel: NotifyChannel{
				DingTalk: DingTalk{
					Enable:   true,
					Webhook:  "https://dingtalk.com/robot/send?token='abc123,xyz789'",
					Receiver: []string{"dev-group", "ops-group"},
				},
				SMS: SMS{
					Enable:    false,
					PhoneList: []string{"13800138000", "13900139000"},
				},
			},
			HistoryRetention: HistoryRetention{
				Day:         30,
				StoreEngine: "influxdb",
				ConnAddr: ConnAddr{
					Addr: "10.0.4.50",
					Port: 8086,
					DB:   "gw_monitor",
					Auth: Auth{
						User: "monitor_admin",
						Pass: "admin's@2026",
					},
				},
			},
		},
		Storage: Storage{
			RedisCluster: RedisCluster{
				Enable:       true,
				MasterNodes:  []string{"10.0.5.10:6379", "10.0.5.11:6379"},
				ReplicaNodes: []string{"10.0.5.12:6379"},
				Password:     "Redis@123456,Secure",
				DBIndex:      0,
				PoolSize:     200,
				ExpireMap: map[string]int{
					"token":       86400,
					"cache_goods": 3600,
					"hot_order":   600,
				},
			},
			MySQL: MySQL{
				Main: MySQLNode{
					DSN:     "root:Mysql'Pass@tcp(10.0.6.30:3306)/biz_db",
					MaxOpen: 100,
					MaxIdle: 20,
					Tables:  []string{"t_user", "t_order", "t_pay_log"},
				},
				Backup: MySQLNode{
					DSN:        "root:123456@tcp(10.0.6.31:3306)/biz_db",
					ReadOnly:   true,
					SlaveDelay: 0.5,
				},
			},
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("cfg mismatch\nwant: %+v\n got: %+v", want, cfg)
	}
}
