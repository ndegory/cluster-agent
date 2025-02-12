package workers

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	log "github.com/sirupsen/logrus"

	app "github.com/appdynamics/cluster-agent/appd"
	m "github.com/appdynamics/cluster-agent/models"

	"github.com/google/uuid"
)

const (
	DRILL_DOWN_URL_TEMPLATE string = "%s#/location=ANALYTICS_ADQL_SEARCH&searchId=%d"
	BASE_PATH               string = "Application Infrastructure Performance|%s|Custom Metrics|Cluster Stats|"
)

type AdqlSearchWorker struct {
	Bag         *m.AppDBag
	SearchCache map[string]m.AdqlSearch
	Logger      *log.Logger
}

func NewAdqlSearchWorker(bag *m.AppDBag, l *log.Logger) AdqlSearchWorker {
	aw := AdqlSearchWorker{Bag: bag, SearchCache: make(map[string]m.AdqlSearch), Logger: l}
	aw.CacheSearches()
	return aw
}

func (aw *AdqlSearchWorker) GetSearch(metricPath string) string {
	search := ""
	arr := strings.Split(metricPath, "|")
	metricName := arr[len(arr)-1]
	fullName := aw.buildFullMetricName(metricName)

	if searchObj, ok := aw.SearchCache[fullName]; ok {
		//		fmt.Printf("Search object is in the cache. ID %d\n", searchObj.ID)
		search = fmt.Sprintf(DRILL_DOWN_URL_TEMPLATE, aw.Bag.RestAPIUrl, searchObj.ID)
	} else {
		//		fmt.Printf("Search object NOT in the cache. Checking map...\n")
		if sObj, ok := aw.getQueryMap()[metricPath]; ok {
			//			fmt.Printf("Search object found in map. Creating...\n")
			obj, err := aw.CreateSearch(&sObj)
			if err != nil {
				aw.Logger.Printf("Unable to save search object. %v\n", err)
			} else {
				aw.SearchCache[fullName] = *obj
				search = fmt.Sprintf(DRILL_DOWN_URL_TEMPLATE, aw.Bag.RestAPIUrl, obj.ID)
				aw.Logger.Printf("Search object created and cached: %s\n", search)
			}
		} else {
			//if not in the queryMap, no search is necessary
			//			fmt.Printf("Search object not requested. Skipping...\n")
		}
	}
	return search
}

func (aw *AdqlSearchWorker) CacheSearches() error {
	rc := app.NewRestClient(aw.Bag, aw.Logger)
	data, err := rc.CallAppDController("restui/analyticsSavedSearches/getAllAnalyticsSavedSearches", "GET", nil)

	if err != nil {
		fmt.Printf("Unable to get the list of saved searches. %v\n", err)
		return err
	}
	var list []m.AdqlSearch
	e := json.Unmarshal(data, &list)
	if e != nil {
		fmt.Printf("Unable to deserialize the list of saved searches. %v\n", e)
		return e
	}

	for _, searchObj := range list {
		name := searchObj.SearchName
		if strings.Contains(name, aw.Bag.AppName) {
			aw.SearchCache[name] = searchObj
		}
	}

	return nil
}

func (aw *AdqlSearchWorker) CreateSearch(searchObj *m.AdqlSearch) (*m.AdqlSearch, error) {
	name := uuid.New().String()

	cols := []string{}

	val := reflect.ValueOf(searchObj.SchemaDef)
	for i := 0; i < val.Type().NumField(); i++ {
		t := val.Type().Field(i)
		jsonTag := t.Tag.Get("json")

		if jsonTag != "" && jsonTag != "-" {
			wrap := fmt.Sprintf("\"%s\"", jsonTag)
			cols = append(cols, wrap)

		}
	}

	jsonStr := fmt.Sprintf(`{"name": "%s", "adqlQueries": ["%s"], "searchType": "SINGLE", "searchMode": "ADVANCED", "viewMode": "DATA", "visualization": "TABLE", "selectedFields": [%s], "widgets": [], "searchName": "%s"}`, name, searchObj.Query, strings.Join(cols, ","), searchObj.SearchName)
	aw.Logger.WithField("payload", jsonStr).Debug("Create search body.")
	body := []byte(jsonStr)

	rc := app.NewRestClient(aw.Bag, aw.Logger)
	data, errSave := rc.CallAppDController("restui/analyticsSavedSearches/createAnalyticsSavedSearch", "POST", body)
	if errSave != nil {
		return nil, fmt.Errorf("Unable to create search %s. %v\n", name, errSave)
	}

	var obj m.AdqlSearch
	e := json.Unmarshal(data, &obj)
	if e != nil {
		fmt.Printf("Unable to deserialize new saved searches. %v\n", e)
		return nil, e
	}

	return &obj, nil
}

func (aw *AdqlSearchWorker) DeleteSearch(searchID int) error {
	arr := []int{searchID}
	body, err := json.Marshal(arr)
	if err != nil {
		return fmt.Errorf("Unable to serialize delete search request %d. %v", searchID, err)
	}

	rc := app.NewRestClient(aw.Bag, aw.Logger)
	_, errSave := rc.CallAppDController("restui/analyticsSavedSearches/deleteAnalyticsSavedSearches", "POST", body)
	if errSave != nil {
		return fmt.Errorf("Unable to delete search %d. %v\n", searchID, errSave)
	}

	return nil
}

func (aw *AdqlSearchWorker) getQueryMap() map[string]m.AdqlSearch {
	var queryMap = map[string]m.AdqlSearch{
		BASE_PATH + "EventError": m.AdqlSearch{SchemaDef: m.EventSchemaDef{}, SearchName: fmt.Sprintf("%s. EventError", aw.Bag.AppName), SchemaName: aw.Bag.EventSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and category = 'error' ORDER BY creationTimestamp DESC", aw.Bag.EventSchemaName, aw.Bag.AppName)},
		BASE_PATH + "EventCount": m.AdqlSearch{SchemaDef: m.EventSchemaDef{}, SearchName: fmt.Sprintf("%s. EventCount", aw.Bag.AppName), SchemaName: aw.Bag.EventSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' ORDER BY creationTimestamp DESC", aw.Bag.EventSchemaName, aw.Bag.AppName)},
		BASE_PATH + "EvictionThreats": m.AdqlSearch{SchemaDef: m.EventSchemaDef{}, SearchName: aw.buildFullMetricName("EvictionThreats"), SchemaName: aw.Bag.EventSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and subCategory = 'eviction' ORDER BY creationTimestamp DESC", aw.Bag.EventSchemaName, aw.Bag.AppName)},
		BASE_PATH + "PodRunning": m.AdqlSearch{SchemaDef: m.PodSchemaDef{}, SearchName: aw.buildFullMetricName("PodRunning"), SchemaName: aw.Bag.PodSchemaName,
			Query: fmt.Sprintf("SELECT * FROM %s where clusterName = '%s' and  phase = 'Running' ORDER BY pickupTimestamp DESC", aw.Bag.PodSchemaName, aw.Bag.AppName)},
		BASE_PATH + "PodFailed": m.AdqlSearch{SchemaDef: m.PodSchemaDef{}, SearchName: aw.buildFullMetricName("PodFailed"), SchemaName: aw.Bag.PodSchemaName,
			Query: fmt.Sprintf("SELECT * FROM %s where clusterName = '%s' and  phase = 'Failed' ORDER BY pickupTimestamp DESC", aw.Bag.PodSchemaName, aw.Bag.AppName)},
		BASE_PATH + "PodPending": m.AdqlSearch{SchemaDef: m.PodSchemaDef{}, SearchName: aw.buildFullMetricName("PodPending"), SchemaName: aw.Bag.PodSchemaName,
			Query: fmt.Sprintf("SELECT * FROM %s where clusterName = '%s' and  phase = 'Pending' ORDER BY pickupTimestamp DESC", aw.Bag.PodSchemaName, aw.Bag.AppName)},
		BASE_PATH + "Evictions": m.AdqlSearch{SchemaDef: m.PodSchemaDef{}, SearchName: aw.buildFullMetricName("Evictions"), SchemaName: aw.Bag.PodSchemaName,
			Query: fmt.Sprintf("SELECT * FROM %s where clusterName = '%s' and  reason = 'Evicted'  ORDER BY pickupTimestamp DESC", aw.Bag.PodSchemaName, aw.Bag.AppName)},
		BASE_PATH + "PodRestarts": m.AdqlSearch{SchemaDef: m.PodSchemaDef{}, SearchName: aw.buildFullMetricName("PodRestarts"), SchemaName: aw.Bag.PodSchemaName,
			Query: fmt.Sprintf("SELECT * FROM %s where clusterName = '%s' and podRestarts > 0 ORDER BY pickupTimestamp DESC", aw.Bag.PodSchemaName, aw.Bag.AppName)},
		BASE_PATH + "MemoryPressureNodes": m.AdqlSearch{SchemaDef: m.NodeSchemaDef{}, SearchName: aw.buildFullMetricName("MemoryPressureNodes"), SchemaName: aw.Bag.NodeSchemaName,
			Query: fmt.Sprintf("SELECT * FROM %s where clusterName = '%s' and  memoryPressure = true ORDER BY pickupTimestamp DESC", aw.Bag.NodeSchemaName, aw.Bag.AppName)},
		BASE_PATH + "DiskPressureNodes": m.AdqlSearch{SchemaDef: m.NodeSchemaDef{}, SearchName: aw.buildFullMetricName("DiskPressureNodes"), SchemaName: aw.Bag.NodeSchemaName,
			Query: fmt.Sprintf("SELECT * FROM %s where clusterName = '%s' and diskPressure = true ORDER BY pickupTimestamp DESC", aw.Bag.NodeSchemaName, aw.Bag.AppName)},
		BASE_PATH + "PodIssues": m.AdqlSearch{SchemaDef: m.EventSchemaDef{}, SearchName: fmt.Sprintf("%s. PodIssues", aw.Bag.AppName), SchemaName: aw.Bag.EventSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and category = 'error' AND subCategory = 'pod' ORDER BY creationTimestamp DESC", aw.Bag.EventSchemaName, aw.Bag.AppName)},
		BASE_PATH + "ImagePullErrors": m.AdqlSearch{SchemaDef: m.EventSchemaDef{}, SearchName: fmt.Sprintf("%s. ImagePullErrors", aw.Bag.AppName), SchemaName: aw.Bag.EventSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and category = 'error' AND subCategory = 'image' ORDER BY creationTimestamp DESC", aw.Bag.EventSchemaName, aw.Bag.AppName)},
		BASE_PATH + "StorageIssues": m.AdqlSearch{SchemaDef: m.EventSchemaDef{}, SearchName: fmt.Sprintf("%s. StorageIssues", aw.Bag.AppName), SchemaName: aw.Bag.EventSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and category = 'error' AND subCategory IN ('storage', 'quota') ORDER BY creationTimestamp DESC", aw.Bag.EventSchemaName, aw.Bag.AppName)},
		BASE_PATH + "MissingDependencies": m.AdqlSearch{SchemaDef: m.ContainerSchemaDef{}, SearchName: fmt.Sprintf("%s. MissingDependencies", aw.Bag.AppName), SchemaName: aw.Bag.ContainerSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and (missingConfigs != '' OR missingSecrets != '') ", aw.Bag.ContainerSchemaName, aw.Bag.AppName)},
		BASE_PATH + "NoConnectivity": m.AdqlSearch{SchemaDef: m.ContainerSchemaDef{}, SearchName: fmt.Sprintf("%s. NoConnectivity", aw.Bag.AppName), SchemaName: aw.Bag.ContainerSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and length(missingServices) > 0 ", aw.Bag.ContainerSchemaName, aw.Bag.AppName)},
		BASE_PATH + "PodOverconsume": m.AdqlSearch{SchemaDef: m.ContainerSchemaDef{}, SearchName: fmt.Sprintf("%s. PodOverconsume", aw.Bag.AppName), SchemaName: aw.Bag.ContainerSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and (consumptionCpu > %d or consumptionMem > %d)", aw.Bag.ContainerSchemaName, aw.Bag.AppName, aw.Bag.OverconsumptionThreshold, aw.Bag.OverconsumptionThreshold)},
		BASE_PATH + "UseCpu": m.AdqlSearch{SchemaDef: m.ContainerSchemaDef{}, SearchName: fmt.Sprintf("%s. UseCpu", aw.Bag.AppName), SchemaName: aw.Bag.ContainerSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and consumptionCpu > %d  ", aw.Bag.ContainerSchemaName, aw.Bag.AppName, aw.Bag.OverconsumptionThreshold)},
		BASE_PATH + "UseMemory": m.AdqlSearch{SchemaDef: m.ContainerSchemaDef{}, SearchName: fmt.Sprintf("%s. UseMemory", aw.Bag.AppName), SchemaName: aw.Bag.ContainerSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and consumptionMem > %d  ", aw.Bag.ContainerSchemaName, aw.Bag.AppName, aw.Bag.OverconsumptionThreshold)},
		BASE_PATH + "NoLimits": m.AdqlSearch{SchemaDef: m.PodSchemaDef{}, SearchName: fmt.Sprintf("%s. NoLimits", aw.Bag.AppName), SchemaName: aw.Bag.PodSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and phase = 'Running' and limitsDefined = false ORDER by namespace, name", aw.Bag.PodSchemaName, aw.Bag.AppName)},
		BASE_PATH + "NoReadinessProbe": m.AdqlSearch{SchemaDef: m.PodSchemaDef{}, SearchName: fmt.Sprintf("%s. NoReadinessProbe", aw.Bag.AppName), SchemaName: aw.Bag.PodSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and phase = 'Running'  and readyProbes = 0 ORDER by namespace, name", aw.Bag.PodSchemaName, aw.Bag.AppName)},
		BASE_PATH + "NoLivenessProbe": m.AdqlSearch{SchemaDef: m.PodSchemaDef{}, SearchName: fmt.Sprintf("%s. NoLivenessProbe", aw.Bag.AppName), SchemaName: aw.Bag.PodSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and phase = 'Running'  and liveProbes = 0 ORDER by namespace, name", aw.Bag.PodSchemaName, aw.Bag.AppName)},
		BASE_PATH + "Privileged": m.AdqlSearch{SchemaDef: m.PodSchemaDef{}, SearchName: fmt.Sprintf("%s. Privileged", aw.Bag.AppName), SchemaName: aw.Bag.PodSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and phase = 'Running'  AND numPrivileged > 0 ORDER by namespace, name", aw.Bag.PodSchemaName, aw.Bag.AppName)},
		BASE_PATH + "JobCount": m.AdqlSearch{SchemaDef: m.PodSchemaDef{}, SearchName: fmt.Sprintf("%s. JobCount", aw.Bag.AppName), SchemaName: aw.Bag.JobSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' ORDER BY startTime DESC", aw.Bag.JobSchemaName, aw.Bag.AppName)},
		BASE_PATH + "JobFailedCount": m.AdqlSearch{SchemaDef: m.PodSchemaDef{}, SearchName: fmt.Sprintf("%s. JobFailedCount", aw.Bag.AppName), SchemaName: aw.Bag.JobSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and failed > 0 ORDER BY startTime DESC", aw.Bag.JobSchemaName, aw.Bag.AppName)},
		BASE_PATH + "ServiceCount": m.AdqlSearch{SchemaDef: m.EpSchemaDef{}, SearchName: fmt.Sprintf("%s. ServiceCount", aw.Bag.AppName), SchemaName: aw.Bag.EpSchemaName,
			Query: fmt.Sprintf("select distinct(name) from %s where clusterName = '%s' ", aw.Bag.EpSchemaName, aw.Bag.AppName)},
		//		BASE_PATH + "EndpointCount": m.AdqlSearch{SchemaDef: m.EpSchemaDef{}, SearchName: fmt.Sprintf("%s. EndpointCount", aw.Bag.AppName), SchemaName: aw.Bag.EpSchemaName,
		//			Query: fmt.Sprintf("select * from %s where clusterName = '%s' ", aw.Bag.EpSchemaName, aw.Bag.AppName)},
		BASE_PATH + "EPNotReadyCount": m.AdqlSearch{SchemaDef: m.EpSchemaDef{}, SearchName: fmt.Sprintf("%s. EPNotReadyCount", aw.Bag.AppName), SchemaName: aw.Bag.EpSchemaName,
			Query: fmt.Sprintf("select * from %s where length(notReadyIPs) > 0 AND clusterName = '%s' ", aw.Bag.EpSchemaName, aw.Bag.AppName)},
		BASE_PATH + "DeployCount": m.AdqlSearch{SchemaDef: m.DeploySchemaDef{}, SearchName: fmt.Sprintf("%s. DeployCount", aw.Bag.AppName), SchemaName: aw.Bag.DeploySchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and deploymentType = '%s' ORDER by namespace, name", aw.Bag.DeploySchemaName, aw.Bag.AppName, m.DEPLOYMENT_TYPE_DEPLOYMENT)},
		BASE_PATH + "RsCount": m.AdqlSearch{SchemaDef: m.DeploySchemaDef{}, SearchName: fmt.Sprintf("%s. RsCount", aw.Bag.AppName), SchemaName: aw.Bag.DeploySchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and deploymentType = '%s' ORDER by namespace, name", aw.Bag.DeploySchemaName, aw.Bag.AppName, m.DEPLOYMENT_TYPE_RS)},
		BASE_PATH + "DaemonCount": m.AdqlSearch{SchemaDef: m.DeploySchemaDef{}, SearchName: fmt.Sprintf("%s. DaemonCount", aw.Bag.AppName), SchemaName: aw.Bag.DeploySchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and deploymentType = '%s' ORDER by namespace, name", aw.Bag.DeploySchemaName, aw.Bag.AppName, m.DEPLOYMENT_TYPE_DS)},
		BASE_PATH + "NamespaceNoQuotas": m.AdqlSearch{SchemaDef: m.NsSchemaDef{}, SearchName: fmt.Sprintf("%s. NamespaceNoQuotas", aw.Bag.AppName), SchemaName: aw.Bag.NsSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and quotas = 0 ORDER by name", aw.Bag.NsSchemaName, aw.Bag.AppName)},
		BASE_PATH + "NamespaceCount": m.AdqlSearch{SchemaDef: m.NsSchemaDef{}, SearchName: fmt.Sprintf("%s. NamespaceCount", aw.Bag.AppName), SchemaName: aw.Bag.NsSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' ORDER by name", aw.Bag.NsSchemaName, aw.Bag.AppName)},
		BASE_PATH + "OrphanEndpoint": m.AdqlSearch{SchemaDef: m.EpSchemaDef{}, SearchName: fmt.Sprintf("%s. OrphanEndpoint", aw.Bag.AppName), SchemaName: aw.Bag.EpSchemaName,
			Query: fmt.Sprintf("select * from %s where clusterName = '%s' and isOrphan = true", aw.Bag.EpSchemaName, aw.Bag.AppName)},
	}

	return queryMap
}

func (aw *AdqlSearchWorker) buildFullMetricName(metricName string) string {
	return fmt.Sprintf("%s. %s", aw.Bag.AppName, metricName)
}
