// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        v5.29.1
// source: msg.proto

package pb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type LockState int32

const (
	LockState_LOCK_INVALID        LockState = 0
	LockState_LOCK_ACQUIRED       LockState = 1
	LockState_LOCK_ACQUIRE_FAILED LockState = 2
	LockState_LOCK_RELEASED       LockState = 3
	LockState_LOCK_RELEASE_FAILED LockState = 4
	LockState_LOCK_TRY_ACQUIRE    LockState = 5
	LockState_LOCK_TRY_RELEASE    LockState = 6
)

// Enum value maps for LockState.
var (
	LockState_name = map[int32]string{
		0: "LOCK_INVALID",
		1: "LOCK_ACQUIRED",
		2: "LOCK_ACQUIRE_FAILED",
		3: "LOCK_RELEASED",
		4: "LOCK_RELEASE_FAILED",
		5: "LOCK_TRY_ACQUIRE",
		6: "LOCK_TRY_RELEASE",
	}
	LockState_value = map[string]int32{
		"LOCK_INVALID":        0,
		"LOCK_ACQUIRED":       1,
		"LOCK_ACQUIRE_FAILED": 2,
		"LOCK_RELEASED":       3,
		"LOCK_RELEASE_FAILED": 4,
		"LOCK_TRY_ACQUIRE":    5,
		"LOCK_TRY_RELEASE":    6,
	}
)

func (x LockState) Enum() *LockState {
	p := new(LockState)
	*p = x
	return p
}

func (x LockState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LockState) Descriptor() protoreflect.EnumDescriptor {
	return file_msg_proto_enumTypes[0].Descriptor()
}

func (LockState) Type() protoreflect.EnumType {
	return &file_msg_proto_enumTypes[0]
}

func (x LockState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LockState.Descriptor instead.
func (LockState) EnumDescriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{0}
}

type ControlMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key       string    `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Deadline  int64     `protobuf:"varint,2,opt,name=deadline,proto3" json:"deadline,omitempty"` // Use timestamp or duration in nanoseconds
	LockState LockState `protobuf:"varint,3,opt,name=lockState,proto3,enum=netsync.pb.LockState" json:"lockState,omitempty"`
}

func (x *ControlMessage) Reset() {
	*x = ControlMessage{}
	mi := &file_msg_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ControlMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ControlMessage) ProtoMessage() {}

func (x *ControlMessage) ProtoReflect() protoreflect.Message {
	mi := &file_msg_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ControlMessage.ProtoReflect.Descriptor instead.
func (*ControlMessage) Descriptor() ([]byte, []int) {
	return file_msg_proto_rawDescGZIP(), []int{0}
}

func (x *ControlMessage) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *ControlMessage) GetDeadline() int64 {
	if x != nil {
		return x.Deadline
	}
	return 0
}

func (x *ControlMessage) GetLockState() LockState {
	if x != nil {
		return x.LockState
	}
	return LockState_LOCK_INVALID
}

var File_msg_proto protoreflect.FileDescriptor

var file_msg_proto_rawDesc = []byte{
	0x0a, 0x09, 0x6d, 0x73, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0a, 0x6e, 0x65, 0x74,
	0x73, 0x79, 0x6e, 0x63, 0x2e, 0x70, 0x62, 0x22, 0x73, 0x0a, 0x0e, 0x43, 0x6f, 0x6e, 0x74, 0x72,
	0x6f, 0x6c, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x1a, 0x0a, 0x08, 0x64,
	0x65, 0x61, 0x64, 0x6c, 0x69, 0x6e, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x64,
	0x65, 0x61, 0x64, 0x6c, 0x69, 0x6e, 0x65, 0x12, 0x33, 0x0a, 0x09, 0x6c, 0x6f, 0x63, 0x6b, 0x53,
	0x74, 0x61, 0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x15, 0x2e, 0x6e, 0x65, 0x74,
	0x73, 0x79, 0x6e, 0x63, 0x2e, 0x70, 0x62, 0x2e, 0x4c, 0x6f, 0x63, 0x6b, 0x53, 0x74, 0x61, 0x74,
	0x65, 0x52, 0x09, 0x6c, 0x6f, 0x63, 0x6b, 0x53, 0x74, 0x61, 0x74, 0x65, 0x2a, 0xa1, 0x01, 0x0a,
	0x09, 0x4c, 0x6f, 0x63, 0x6b, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x10, 0x0a, 0x0c, 0x4c, 0x4f,
	0x43, 0x4b, 0x5f, 0x49, 0x4e, 0x56, 0x41, 0x4c, 0x49, 0x44, 0x10, 0x00, 0x12, 0x11, 0x0a, 0x0d,
	0x4c, 0x4f, 0x43, 0x4b, 0x5f, 0x41, 0x43, 0x51, 0x55, 0x49, 0x52, 0x45, 0x44, 0x10, 0x01, 0x12,
	0x17, 0x0a, 0x13, 0x4c, 0x4f, 0x43, 0x4b, 0x5f, 0x41, 0x43, 0x51, 0x55, 0x49, 0x52, 0x45, 0x5f,
	0x46, 0x41, 0x49, 0x4c, 0x45, 0x44, 0x10, 0x02, 0x12, 0x11, 0x0a, 0x0d, 0x4c, 0x4f, 0x43, 0x4b,
	0x5f, 0x52, 0x45, 0x4c, 0x45, 0x41, 0x53, 0x45, 0x44, 0x10, 0x03, 0x12, 0x17, 0x0a, 0x13, 0x4c,
	0x4f, 0x43, 0x4b, 0x5f, 0x52, 0x45, 0x4c, 0x45, 0x41, 0x53, 0x45, 0x5f, 0x46, 0x41, 0x49, 0x4c,
	0x45, 0x44, 0x10, 0x04, 0x12, 0x14, 0x0a, 0x10, 0x4c, 0x4f, 0x43, 0x4b, 0x5f, 0x54, 0x52, 0x59,
	0x5f, 0x41, 0x43, 0x51, 0x55, 0x49, 0x52, 0x45, 0x10, 0x05, 0x12, 0x14, 0x0a, 0x10, 0x4c, 0x4f,
	0x43, 0x4b, 0x5f, 0x54, 0x52, 0x59, 0x5f, 0x52, 0x45, 0x4c, 0x45, 0x41, 0x53, 0x45, 0x10, 0x06,
	0x42, 0x2f, 0x5a, 0x2d, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x4d,
	0x4f, 0x48, 0x41, 0x4e, 0x4b, 0x55, 0x4d, 0x41, 0x52, 0x2d, 0x49, 0x54, 0x2f, 0x67, 0x6f, 0x2d,
	0x6c, 0x69, 0x62, 0x70, 0x32, 0x70, 0x2d, 0x6e, 0x65, 0x74, 0x73, 0x79, 0x6e, 0x63, 0x2f, 0x70,
	0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_msg_proto_rawDescOnce sync.Once
	file_msg_proto_rawDescData = file_msg_proto_rawDesc
)

func file_msg_proto_rawDescGZIP() []byte {
	file_msg_proto_rawDescOnce.Do(func() {
		file_msg_proto_rawDescData = protoimpl.X.CompressGZIP(file_msg_proto_rawDescData)
	})
	return file_msg_proto_rawDescData
}

var file_msg_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_msg_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_msg_proto_goTypes = []any{
	(LockState)(0),         // 0: netsync.pb.LockState
	(*ControlMessage)(nil), // 1: netsync.pb.ControlMessage
}
var file_msg_proto_depIdxs = []int32{
	0, // 0: netsync.pb.ControlMessage.lockState:type_name -> netsync.pb.LockState
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_msg_proto_init() }
func file_msg_proto_init() {
	if File_msg_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_msg_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_msg_proto_goTypes,
		DependencyIndexes: file_msg_proto_depIdxs,
		EnumInfos:         file_msg_proto_enumTypes,
		MessageInfos:      file_msg_proto_msgTypes,
	}.Build()
	File_msg_proto = out.File
	file_msg_proto_rawDesc = nil
	file_msg_proto_goTypes = nil
	file_msg_proto_depIdxs = nil
}