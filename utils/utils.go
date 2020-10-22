package utils

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var RPCTimeout = 20 * time.Second
//新建一个Grpc连接 , address = NodeHost:port
func CreateGrpcConnection(ctx context.Context, c client.Client, pod *v1.Pod, port int) (*grpc.ClientConn, error){
	nodeName := pod.Spec.NodeName

	var node v1.Node
	err := c.Get(ctx , types.NamespacedName{
		Name : nodeName,
	}, &node)

	if err != nil{
		fmt.Printf("Failed to get node with nodename : %s , error : %s\n", nodeName, err.Error())
		return nil ,err
	}

	target := fmt.Sprintf("%s:%d", node.Status.Addresses[0].Address, port)
    	fmt.Printf("Creating Grpc connection to %s:%d\n",node.Status.Addresses[0].Address , port)
	//建立连接
	conn , err := grpc.Dial(target ,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(TimeoutClientInterceptor),
	)

	if err != nil{
		fmt.Printf("Failed to dial target address : %s\n", err.Error())
		return nil , err
	}

	return conn, nil

}

// timeout 拦截器
func TimeoutClientInterceptor(ctx context.Context, method string, req, reply interface{},
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	ctx, cancel := context.WithTimeout(ctx, RPCTimeout)
	defer cancel()
	return invoker(ctx, method, req, reply, cc, opts...)
}
