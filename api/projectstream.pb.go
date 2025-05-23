// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.4
// 	protoc        v5.29.3
// source: api/projectstream.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type EmptyRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *EmptyRequest) Reset() {
	*x = EmptyRequest{}
	mi := &file_api_projectstream_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *EmptyRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EmptyRequest) ProtoMessage() {}

func (x *EmptyRequest) ProtoReflect() protoreflect.Message {
	mi := &file_api_projectstream_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EmptyRequest.ProtoReflect.Descriptor instead.
func (*EmptyRequest) Descriptor() ([]byte, []int) {
	return file_api_projectstream_proto_rawDescGZIP(), []int{0}
}

type ProjectData struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ProjectName   string                 `protobuf:"bytes,1,opt,name=project_name,json=projectName,proto3" json:"project_name,omitempty"`
	OrgName       string                 `protobuf:"bytes,2,opt,name=org_name,json=orgName,proto3" json:"org_name,omitempty"`
	Status        string                 `protobuf:"bytes,3,opt,name=status,proto3" json:"status,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ProjectData) Reset() {
	*x = ProjectData{}
	mi := &file_api_projectstream_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProjectData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProjectData) ProtoMessage() {}

func (x *ProjectData) ProtoReflect() protoreflect.Message {
	mi := &file_api_projectstream_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProjectData.ProtoReflect.Descriptor instead.
func (*ProjectData) Descriptor() ([]byte, []int) {
	return file_api_projectstream_proto_rawDescGZIP(), []int{1}
}

func (x *ProjectData) GetProjectName() string {
	if x != nil {
		return x.ProjectName
	}
	return ""
}

func (x *ProjectData) GetOrgName() string {
	if x != nil {
		return x.OrgName
	}
	return ""
}

func (x *ProjectData) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

type ProjectUpdate struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Projects      []*ProjectEntry        `protobuf:"bytes,1,rep,name=projects,proto3" json:"projects,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ProjectUpdate) Reset() {
	*x = ProjectUpdate{}
	mi := &file_api_projectstream_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProjectUpdate) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProjectUpdate) ProtoMessage() {}

func (x *ProjectUpdate) ProtoReflect() protoreflect.Message {
	mi := &file_api_projectstream_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProjectUpdate.ProtoReflect.Descriptor instead.
func (*ProjectUpdate) Descriptor() ([]byte, []int) {
	return file_api_projectstream_proto_rawDescGZIP(), []int{2}
}

func (x *ProjectUpdate) GetProjects() []*ProjectEntry {
	if x != nil {
		return x.Projects
	}
	return nil
}

type ProjectEntry struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Key           string                 `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Data          *ProjectData           `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ProjectEntry) Reset() {
	*x = ProjectEntry{}
	mi := &file_api_projectstream_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProjectEntry) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProjectEntry) ProtoMessage() {}

func (x *ProjectEntry) ProtoReflect() protoreflect.Message {
	mi := &file_api_projectstream_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProjectEntry.ProtoReflect.Descriptor instead.
func (*ProjectEntry) Descriptor() ([]byte, []int) {
	return file_api_projectstream_proto_rawDescGZIP(), []int{3}
}

func (x *ProjectEntry) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *ProjectEntry) GetData() *ProjectData {
	if x != nil {
		return x.Data
	}
	return nil
}

var File_api_projectstream_proto protoreflect.FileDescriptor

var file_api_projectstream_proto_rawDesc = string([]byte{
	0x0a, 0x17, 0x61, 0x70, 0x69, 0x2f, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x73, 0x74, 0x72,
	0x65, 0x61, 0x6d, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0d, 0x70, 0x72, 0x6f, 0x6a, 0x65,
	0x63, 0x74, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x22, 0x0e, 0x0a, 0x0c, 0x45, 0x6d, 0x70, 0x74,
	0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x63, 0x0a, 0x0b, 0x50, 0x72, 0x6f, 0x6a,
	0x65, 0x63, 0x74, 0x44, 0x61, 0x74, 0x61, 0x12, 0x21, 0x0a, 0x0c, 0x70, 0x72, 0x6f, 0x6a, 0x65,
	0x63, 0x74, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x70,
	0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x6f, 0x72,
	0x67, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6f, 0x72,
	0x67, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0x48, 0x0a,
	0x0d, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x37,
	0x0a, 0x08, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x1b, 0x2e, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d,
	0x2e, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x08, 0x70,
	0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x73, 0x22, 0x50, 0x0a, 0x0c, 0x50, 0x72, 0x6f, 0x6a, 0x65,
	0x63, 0x74, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x2e, 0x0a, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63,
	0x74, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x2e, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x44,
	0x61, 0x74, 0x61, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x32, 0x65, 0x0a, 0x0e, 0x50, 0x72, 0x6f,
	0x6a, 0x65, 0x63, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x53, 0x0a, 0x14, 0x53,
	0x74, 0x72, 0x65, 0x61, 0x6d, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x55, 0x70, 0x64, 0x61,
	0x74, 0x65, 0x73, 0x12, 0x1b, 0x2e, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x73, 0x74, 0x72,
	0x65, 0x61, 0x6d, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x1c, 0x2e, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d,
	0x2e, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x30, 0x01,
	0x42, 0x08, 0x5a, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
})

var (
	file_api_projectstream_proto_rawDescOnce sync.Once
	file_api_projectstream_proto_rawDescData []byte
)

func file_api_projectstream_proto_rawDescGZIP() []byte {
	file_api_projectstream_proto_rawDescOnce.Do(func() {
		file_api_projectstream_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_api_projectstream_proto_rawDesc), len(file_api_projectstream_proto_rawDesc)))
	})
	return file_api_projectstream_proto_rawDescData
}

var file_api_projectstream_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_api_projectstream_proto_goTypes = []any{
	(*EmptyRequest)(nil),  // 0: projectstream.EmptyRequest
	(*ProjectData)(nil),   // 1: projectstream.ProjectData
	(*ProjectUpdate)(nil), // 2: projectstream.ProjectUpdate
	(*ProjectEntry)(nil),  // 3: projectstream.ProjectEntry
}
var file_api_projectstream_proto_depIdxs = []int32{
	3, // 0: projectstream.ProjectUpdate.projects:type_name -> projectstream.ProjectEntry
	1, // 1: projectstream.ProjectEntry.data:type_name -> projectstream.ProjectData
	0, // 2: projectstream.ProjectService.StreamProjectUpdates:input_type -> projectstream.EmptyRequest
	2, // 3: projectstream.ProjectService.StreamProjectUpdates:output_type -> projectstream.ProjectUpdate
	3, // [3:4] is the sub-list for method output_type
	2, // [2:3] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_api_projectstream_proto_init() }
func file_api_projectstream_proto_init() {
	if File_api_projectstream_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_api_projectstream_proto_rawDesc), len(file_api_projectstream_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_api_projectstream_proto_goTypes,
		DependencyIndexes: file_api_projectstream_proto_depIdxs,
		MessageInfos:      file_api_projectstream_proto_msgTypes,
	}.Build()
	File_api_projectstream_proto = out.File
	file_api_projectstream_proto_goTypes = nil
	file_api_projectstream_proto_depIdxs = nil
}
