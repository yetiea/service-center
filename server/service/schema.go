//Copyright 2017 Huawei Technologies Co., Ltd
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
package service

import (
	"github.com/ServiceComb/service-center/pkg/util"
	apt "github.com/ServiceComb/service-center/server/core"
	pb "github.com/ServiceComb/service-center/server/core/proto"
	"github.com/ServiceComb/service-center/server/core/registry"
	"github.com/ServiceComb/service-center/server/core/registry/store"
	"github.com/ServiceComb/service-center/server/infra/quota"
	serviceUtil "github.com/ServiceComb/service-center/server/service/util"
	"golang.org/x/net/context"
	"errors"
	"github.com/ServiceComb/service-center/server/rest/controller/v4"
	"strings"
)

func (s *ServiceController) GetSchemaInfo(ctx context.Context, in *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	if in == nil || len(in.ServiceId) == 0 || len(in.SchemaId) == 0 {
		util.Logger().Errorf(nil, "get schema failed: invalid params.")
		return &pb.GetSchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Invalid request path."),
		}, nil
	}

	err := apt.Validate(in)
	if err != nil {
		util.Logger().Errorf(nil, "get schema failed, serviceId %s, schemaId %s: invalid params.", in.ServiceId, in.SchemaId)
		return &pb.GetSchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, err.Error()),
		}, nil
	}

	tenant := util.ParseTenantProject(ctx)

	opts := serviceUtil.QueryOptions(serviceUtil.WithNoCache(in.NoCache))

	if !serviceUtil.ServiceExist(ctx, tenant, in.ServiceId, opts...) {
		util.Logger().Errorf(nil, "get schema failed, serviceId %s, schemaId %s: service not exist.", in.ServiceId, in.SchemaId)
		return &pb.GetSchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Service does not exist."),
		}, nil
	}

	key := apt.GenerateServiceSchemaKey(tenant, in.ServiceId, in.SchemaId)
	opts = append(opts, registry.WithStrKey(key))
	resp, errDo := store.Store().Schema().Search(ctx, opts...)
	if errDo != nil {
		util.Logger().Errorf(errDo, "get schema failed, serviceId %s, schemaId %s: get schema info failed.", in.ServiceId, in.SchemaId)
		return &pb.GetSchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Get schema info failed."),
		}, errDo
	}
	if resp.Count == 0 {
		util.Logger().Errorf(errDo, "get schema failed, serviceId %s, schemaId %s: schema not exists.", in.ServiceId, in.SchemaId)
		return &pb.GetSchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Do not have this schema info."),
		}, nil
	}
	return &pb.GetSchemaResponse{
		Response: pb.CreateResponse(pb.Response_SUCCESS, "Get schema info successfully."),
		Schema:   util.BytesToStringWithNoCopy(resp.Kvs[0].Value),
	}, nil
}

func (s *ServiceController) DeleteSchema(ctx context.Context, request *pb.DeleteSchemaRequest) (*pb.DeleteSchemaResponse, error) {
	if request == nil || len(request.ServiceId) == 0 || len(request.SchemaId) == 0 {
		util.Logger().Errorf(nil, "delete schema failded: invalid params.")
		return &pb.DeleteSchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Invalid request path."),
		}, nil
	}
	err := apt.Validate(request)
	if err != nil {
		util.Logger().Errorf(err, "delete schema failded, serviceId %s, schemaId %s: invalid params.", request.ServiceId, request.SchemaId)
		return &pb.DeleteSchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, err.Error()),
		}, nil
	}
	tenant := util.ParseTenantProject(ctx)

	if !serviceUtil.ServiceExist(ctx, tenant, request.ServiceId) {
		util.Logger().Errorf(nil, "delete schema failded, serviceId %s, schemaId %s: service not exist.", request.ServiceId, request.SchemaId)
		return &pb.DeleteSchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Service does not exist."),
		}, nil
	}

	key := apt.GenerateServiceSchemaKey(tenant, request.ServiceId, request.SchemaId)
	exist, err := serviceUtil.CheckSchemaInfoExist(ctx, key)
	if err != nil {
		util.Logger().Errorf(err, "delete schema failded, serviceId %s, schemaId %s: get schema failed.", request.ServiceId, request.SchemaId)
		return &pb.DeleteSchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Schema info does not exist."),
		}, err
	}
	if !exist {
		util.Logger().Errorf(nil, "delete schema failded, serviceId %s, schemaId %s: schema not exist.", request.ServiceId, request.SchemaId)
		return &pb.DeleteSchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Schema info does not exist."),
		}, nil
	}
	_, errDo := registry.GetRegisterCenter().Do(ctx, registry.DEL, registry.WithStrKey(key))
	if errDo != nil {
		util.Logger().Errorf(errDo, "delete schema failded, serviceId %s, schemaId %s: delete schema from etcd faild.", request.ServiceId, request.SchemaId)
		return &pb.DeleteSchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Delete schema info failed."),
		}, errDo
	}
	util.Logger().Infof("delete schema info successfully.%s", request.SchemaId)
	return &pb.DeleteSchemaResponse{
		Response: pb.CreateResponse(pb.Response_SUCCESS, "Delete schema info successfully."),
	}, nil
}

func (s *ServiceController) ModifySchemas(ctx context.Context, request *pb.ModifySchemasRequest) (*pb.ModifySchemasResponse, error) {
	err := apt.Validate(request)
	if err != nil {
		util.Logger().Errorf(err, "modify schemas failded: invalid params.")
		return &pb.ModifySchemasResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Invalid request."),
		}, nil
	}
	serviceId := request.ServiceId

	tenant := util.ParseTenantProject(ctx)

	service, err := serviceUtil.GetService(ctx, tenant, serviceId)
	if err != nil {
		util.Logger().Errorf(err, "modify schemas failded: get service failed. %s", serviceId)
		return &pb.ModifySchemasResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Invalid request."),
		}, err
	}
	if service == nil {
		util.Logger().Errorf(nil, "modify schemas failded: service not exist. %s", serviceId)
		return &pb.ModifySchemasResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Service not exist."),
		}, nil
	}

	err, isInnErr := modifySchemas(ctx, tenant, service, request.Schemas)
	if err != nil {
		if isInnErr {
			return &pb.ModifySchemasResponse{
				Response: pb.CreateResponse(pb.Response_FAIL, err.Error()),
			}, err
		}
		return &pb.ModifySchemasResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, err.Error()),
		}, nil
	}

	return &pb.ModifySchemasResponse{
		Response: pb.CreateResponse(pb.Response_SUCCESS, "modify schemas info successfully."),
	}, nil
}

func modifySchemas(ctx context.Context, tenant string, service *pb.MicroService, schemas []*pb.Schema) (err error, innerErr bool) {
	if !isExistSchemaId(service, schemas) {
		return errors.New("exist non-exist schemaId"), false
	}

	serviceId := service.ServiceId
	schemasInDataBase, err := GetSchemasFromDataBase(ctx, tenant, serviceId)
	if err != nil {
		util.Logger().Errorf(nil, "modify schema failed: get schema from database failed, %s", serviceId)
		return errors.New("exist not exist schemaId"),  true
	}

	needUpdateSchemaList := make([]*pb.Schema, 0, len(schemas))
	needAddSchemaList := make([]*pb.Schema, 0, len(schemas))

	if len(schemasInDataBase) == 0 {
		needAddSchemaList = schemas
	} else {
		for _, schema := range schemas {
			exist := false
			for _, schemaInner := range schemasInDataBase {
				if schema.SchemaId == schemaInner.SchemaId {
					needUpdateSchemaList = append(needUpdateSchemaList, schema)
					exist = true
					break
				}
			}
			if !exist {
				needAddSchemaList = append(needAddSchemaList, schema)
			}
		}
	}

	pluginOps := []registry.PluginOp{}
	switch v4.RunMode {
	case "dev":
		needDeleteSchemaList := make([]*pb.Schema, 0, len(schemasInDataBase))
		for _, schemasInner := range schemasInDataBase {
			exist := false
			for _, schema := range schemas {
				if schema.SchemaId == schemasInner.SchemaId {
					exist = true
					break
				}
			}
			if !exist {
				needDeleteSchemaList = append(needDeleteSchemaList, schemasInner)
			}
		}

		quotaSize := len(needAddSchemaList) - len(needDeleteSchemaList)
		if quotaSize > 0 {
			_, ok, err := quota.QuotaPlugins[quota.QuataType]().Apply4Quotas(ctx, quota.SCHEMAQuotaType, tenant, serviceId, int16(quotaSize))
			if err != nil {
				util.Logger().Errorf(err, "Add schema info failed, check resource num failed, %s", serviceId)
				return err, true
			}
			if !ok {
				util.Logger().Errorf(err, "Add schema info failed, reach the max size of schema, %s", serviceId)
				return errors.New("reach the max size of schema"), false
			}
		}

		for _, schema := range needUpdateSchemaList {
			util.Logger().Infof("update schema %v", schema)
			opts:= schemaWithDatabaseOpera(registry.OpPut, tenant, serviceId, schema)
			pluginOps = append(pluginOps, opts...)
		}
		for _, schema := range needDeleteSchemaList {
			util.Logger().Infof("delete not exist schema %v", schema)
			opts:= schemaWithDatabaseOpera(registry.OpDel, tenant, serviceId, schema)
			pluginOps = append(pluginOps, opts...)
		}
	case "prod":
		quotaSize := len(needAddSchemaList)
		if quotaSize > 0 {
			_, ok, err := quota.QuotaPlugins[quota.QuataType]().Apply4Quotas(ctx, quota.SCHEMAQuotaType, tenant, serviceId, int16(quotaSize))
			if err != nil {
				util.Logger().Errorf(err, "Add schema info failed, check resource num failed, %s", serviceId)
				return err, true
			}
			if !ok {
				util.Logger().Errorf(err, "Add schema info failed, reach the max size of schema, %s", serviceId)
				return errors.New("reach the max size of schema"), false
			}
		}
		schemasFromDatabase, err := GetSchemasSummaryFromDataBase(ctx, tenant, serviceId)
		if err != nil {
			util.Logger().Errorf(err, "get schema summary failed")
			return errors.New("run mode is prod, schema more exist, can't change"), false
		}
		for _, schema := range needUpdateSchemaList {
			exist := false
			for _, schemaDatabase := range schemasFromDatabase {
				if schema.SchemaId == schemaDatabase.SchemaId {
					exist = true
					break
				}
			}
			if !exist {
				keySummary := apt.GenerateServiceSchemaSummaryKey(tenant, serviceId, schema.SchemaId)
				opt := registry.OpPut(registry.WithStrKey(keySummary), registry.WithStrValue(schema.Summary))
				pluginOps = append(pluginOps, opt)
			}
		}
		if len(needUpdateSchemaList) > 0 && len(pluginOps) == 0 {
			util.Logger().Errorf(nil, "run mode is prod, schema more exist, can't change.%v", needUpdateSchemaList)
			return errors.New("run mode is prod, schema more exist, can't change"), false
		}
	}

	for _, schema := range needAddSchemaList {
		util.Logger().Infof("add new schema %v", schema)
		opts:= schemaWithDatabaseOpera(registry.OpPut, tenant, service.ServiceId, schema)
		pluginOps = append(pluginOps, opts...)
	}

	if len(pluginOps) != 0 {
		return registry.BatchCommit(ctx, pluginOps), true
	}
	return nil, false
}

func isExistSchemaId(service *pb.MicroService, schemas []*pb.Schema) bool{
	seriviceSchemaIds := service.Schemas
	for _, schema := range schemas {
		if !containsValueInSlice(seriviceSchemaIds, schema.SchemaId) {
			util.Logger().Errorf(nil, "modify schemas failed: exist not exist schemaId, %s", schema.SchemaId)
			return false
		}
	}
	return true
}

func schemaWithDatabaseOpera(invoke registry.Operation, tenant string, serviceId string, schema *pb.Schema) []registry.PluginOp{
	pluginOps := []registry.PluginOp{}
	key := apt.GenerateServiceSchemaKey(tenant, serviceId, schema.SchemaId)
	opt := invoke(registry.WithStrKey(key), registry.WithStrValue(schema.Schema))
	pluginOps = append(pluginOps, opt)
	keySummary := apt.GenerateServiceSchemaSummaryKey(tenant, serviceId, schema.SchemaId)
	opt = invoke(registry.WithStrKey(keySummary), registry.WithStrValue(schema.Summary))
	pluginOps = append(pluginOps, opt)
	return pluginOps
}

func GetSchemasFromDataBase(ctx context.Context, tenant string, serviceId string) ([]*pb.Schema, error) {
	schemas := []*pb.Schema{}
	key := apt.GenerateServiceSchemaKey(tenant, serviceId, "")
	util.Logger().Debugf("key is %s", key)
	resp, err := registry.GetRegisterCenter().Do(ctx,
		registry.GET,
		registry.WithPrefix(),
		registry.WithStrKey(key))
	if err != nil {
		util.Logger().Errorf(err, "Get schema of one service failed. %s", serviceId)
		return schemas, err
	}
	for _, kv := range resp.Kvs {
		key := util.BytesToStringWithNoCopy(kv.Key)
		tmp := strings.Split(key, "/")
		schemaId := tmp[len(tmp) - 1]
		schema := util.BytesToStringWithNoCopy(kv.Value)
		schemaStruct := &pb.Schema{
			SchemaId: schemaId,
			Schema: schema,
		}
		schemas = append(schemas, schemaStruct)
	}
	return schemas, nil
}

func GetSchemasSummaryFromDataBase(ctx context.Context, tenant string, serviceId string) ([]*pb.Schema, error) {
	schemas := []*pb.Schema{}
	key := apt.GenerateServiceSchemaSummaryKey(tenant, serviceId, "")
	util.Logger().Debugf("key is %s", key)
	resp, err := store.Store().SchemaSummary().Search(ctx,
		registry.WithPrefix(),
		registry.WithStrKey(key))
	if err != nil {
		util.Logger().Errorf(err, "Get schema of one service failed. %s", serviceId)
		return schemas, err
	}
	for _, kv := range resp.Kvs {
		key := util.BytesToStringWithNoCopy(kv.Key)
		tmp := strings.Split(key, "/")
		schemaId := tmp[len(tmp) - 1]
		summary := util.BytesToStringWithNoCopy(kv.Value)
		schemaStruct := &pb.Schema{
			SchemaId: schemaId,
			Summary: summary,
		}
		schemas = append(schemas, schemaStruct)
	}
	return schemas, nil
}

func (s *ServiceController) ModifySchema(ctx context.Context, request *pb.ModifySchemaRequest) (*pb.ModifySchemaResponse, error) {
	err, rst := s.canModifySchema(ctx, request)
	if err != nil {
		if !rst {
			return &pb.ModifySchemaResponse{
				Response: pb.CreateResponse(pb.Response_FAIL, err.Error()),
			}, nil
		}
		return &pb.ModifySchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Modify schema info failed."),
		}, err
	}
	tenant := util.ParseTenantProject(ctx)
	serviceId := request.ServiceId
	schemaId := request.SchemaId

	_, ok, err := quota.QuotaPlugins[quota.QuataType]().Apply4Quotas(ctx, quota.SCHEMAQuotaType, tenant, serviceId, 1)
	if err != nil {
		util.Logger().Errorf(err, "Add schema info failed, check resource num failed, %s, %s", serviceId, schemaId)
		return &pb.ModifySchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Modify schema info failed, check resource num failed."),
		}, err
	}
	if !ok {
		util.Logger().Errorf(err, "Add schema info failed, reach the max size of shema, %s, %s", serviceId, schemaId)
		return &pb.ModifySchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "reach the max size of shema."),
		}, nil
	}

	key := apt.GenerateServiceSchemaKey(tenant, serviceId, schemaId)
	_, errDo := registry.GetRegisterCenter().Do(ctx,
		registry.PUT,
		registry.WithStrKey(key),
		registry.WithStrValue(request.Schema))
	if errDo != nil {
		util.Logger().Errorf(errDo, "update schema failded, serviceId %s, schemaId %s: commit schema into etcd failed.", serviceId, schemaId)
		return &pb.ModifySchemaResponse{
			Response: pb.CreateResponse(pb.Response_FAIL, "Modify schema info failed."),
		}, errDo
	}
	util.Logger().Infof("update schema success: serviceId %s, schemaId %s.", serviceId, schemaId)
	return &pb.ModifySchemaResponse{
		Response: pb.CreateResponse(pb.Response_SUCCESS, "Modify schema info success."),
	}, nil

}

func (s *ServiceController) canModifySchema(ctx context.Context, request *pb.ModifySchemaRequest) (error, bool) {
	serviceId := request.ServiceId
	schemaId := request.SchemaId
	if len(schemaId) == 0 || len(serviceId) == 0 {
		util.Logger().Errorf(nil, "update schema failded: invalid params.")
		return errors.New("invalid request params"), false
	}
	err := apt.Validate(request)
	if err != nil {
		util.Logger().Errorf(err, "update schema failded, serviceId %s, schemaId %s: invalid params.", serviceId, schemaId)
		return err, false
	}
	tenant := util.ParseTenantProject(ctx)
	service, err := serviceUtil.GetService(ctx, tenant, serviceId)
	if err != nil {
		util.Logger().Errorf(err, "update schema failded, serviceId %s, schemaId %s: get service failed.", serviceId, schemaId)
		return err, true
	}
	if service == nil {
		util.Logger().Errorf(nil, "update schema failded, serviceId %s, schemaId %s: service not exist,%s", serviceId, schemaId)
		return errors.New("service non-exist"), false
	}
	schema := &pb.Schema{
		Schema: request.Schema,
		SchemaId: schemaId,
	}
	if !isExistSchemaId(service, []*pb.Schema{schema}) {
		return errors.New("schemaId non-exist"), false
	}
	return nil, true
}

func containsValueInSlice(in []string, value string) bool {
	if in == nil || len(value) == 0 {
		return false
	}
	for _, i := range in {
		if i == value {
			return true
		}
	}
	return false
}

func getSchemaSummary(ctx context.Context, tenant string, serviceId string, schemaId string) (string, error) {
	key := apt.GenerateServiceSchemaSummaryKey(tenant, serviceId, schemaId)
	resp, err := store.Store().SchemaSummary().Search(ctx,
		registry.WithStrKey(key),
	)
	if err != nil {
		util.Logger().Errorf(err,"get %s schema %s summary failed", serviceId, schemaId)
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", nil
	}
	return util.BytesToStringWithNoCopy(resp.Kvs[0].Value), nil
}