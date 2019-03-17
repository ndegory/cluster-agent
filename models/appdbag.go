package models

type AppDBag struct {
	AgentNamespace              string
	AppName                     string
	TierName                    string
	NodeName                    string
	AppID                       int
	TierID                      int
	NodeID                      int
	Account                     string
	GlobalAccount               string
	AccessKey                   string
	ControllerUrl               string
	ControllerPort              uint16
	RestAPIUrl                  string
	SSLEnabled                  bool
	SystemSSLCert               string
	AgentSSLCert                string
	EventKey                    string
	EventServiceUrl             string
	RestAPICred                 string
	EventAPILimit               int
	PodSchemaName               string
	NodeSchemaName              string
	DeploySchemaName            string
	RSSchemaName                string
	DaemonSchemaName            string
	EventSchemaName             string
	ContainerSchemaName         string
	JobSchemaName               string
	LogSchemaName               string
	DashboardTemplatePath       string
	DashboardSuffix             string
	DashboardDelayMin           int
	JavaAgentVersion            string
	AgentEnvVar                 string
	AgentLabel                  string
	AppDAppLabel                string
	AppDTierLabel               string
	AppDAnalyticsLabel          string
	AgentMountName              string
	AgentMountPath              string
	AppLogMountName             string
	AppLogMountPath             string
	JDKMountName                string
	JDKMountPath                string
	NodeNamePrefix              string
	AnalyticsAgentUrl           string
	AnalyticsAgentImage         string
	AnalyticsAgentContainerName string
	AppDInitContainerName       string
	AppDJavaAttachImage         string
	AppDDotNetAttachImage       string
	AppDNodeJSAttachImage       string
	ProxyInfo                   string
	ProxyUser                   string
	ProxyPass                   string
	InstrumentationMethod       InstrumentationMethod
	InitContainerDir            string
	MetricsSyncInterval         int // Frequency of metrics pushes to the controller, sec
	SnapshotSyncInterval        int // Frequency of snapshot pushes to events api, sec
	AgentServerPort             int
	IncludeNsToInstrument       []string
	ExcludeNsToInstrument       []string
	DeploysToDashboard          []string
	IncludeNodesToInstrument    []string
	ExcludeNodesToInstrument    []string
}
