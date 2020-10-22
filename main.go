package main

/*
	通过chaos daemon 获取container pid
*/
import (
    	"chaos_client/utils"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
    	"k8s.io/client-go/rest"
    	"net/http"
    	"sigs.k8s.io/controller-runtime/pkg/client"
	"fmt"
    	pb "chaos_client/pb"
	"context"
	"sync"
	"errors"
)
//var (
//	podName 		string
//	containerName		string
//	namespace		string
//)
const (
	//chaos daemon port 监听端口
	ChaosDaemonPort = 31767

	kubeconfigPath = "/root/.kube/config"
)
//func init(){
//	flag.StringVar(&namespace , "namespace", "default", "the namespace of target pod")
//	flag.StringVar(&podName , "pod" , "" , "the name of pod which you want to attach")
//	flag.StringVar(&containerName , "container" , "" , "the name of container which you want to attach")
//}
type GrpcChaosDaemonClient struct{
	ChaosDaemonClient 			pb.ChaosDaemonClient
	conn					*grpc.ClientConn
}
func (c *GrpcChaosDaemonClient) Close () error {
	return c.conn.Close()
}
//初始化controller runtime client
func NewRuntimeClient() client.Client{
	//从kubeconfig读取配置
	config , err := rest.InClusterConfig()
	if err != nil{
		fmt.Printf("Failed to init client from kube config : %s\n", err.Error())
		panic(err)
	}
	runtimeClient , err := client.New(config, client.Options{})
	if err != nil{
		fmt.Printf("Failed to new client with config , %s\n", err.Error())
		panic(err)
	}
	return runtimeClient
}

func NewChaosDaemonClient (ctx context.Context, c client.Client , pod *v1.Pod, port int) (*GrpcChaosDaemonClient, error){
	conn ,  err := utils.CreateGrpcConnection(ctx, c , pod , port)

	if err != nil{
		return nil , err
	}

	return &GrpcChaosDaemonClient{
		ChaosDaemonClient: pb.NewChaosDaemonClient(conn),
		conn:	conn,
	}, nil
}

func GetTargetPid(ctx context.Context, c client.Client , namespace string, podName string, containerName string) ([]uint32, error){
	list := &v1.PodList{}
	err := c.List(ctx, list , client.ListOption(&client.ListOptions{Namespace: namespace, Limit: 500}))
	if err != nil{
		return nil ,err
	}
	mutex := new(sync.Mutex)
	pids := make([]uint32, 0, 5)

	g := errgroup.Group{}
	haveContainer := false
	for index := range list.Items{
		pod := &list.Items[index]
		if pod.Name == podName {
			for containerIndex := range pod.Status.ContainerStatuses{
				podContainerName := pod.Status.ContainerStatuses[containerIndex].Name
				containerID 	 := pod.Status.ContainerStatuses[containerIndex].ContainerID

				if podContainerName == containerName{
					haveContainer = true
					//通过daemon client获取pid
					g.Go( func() error{
						pid , err := getPidFromChaosDaemon(ctx, c , pod , containerID)
						if err != nil{
							return err
						}
						mutex.Lock()
						pids = append(pids , pid)
						mutex.Unlock()
						return nil
					})
				}
			}
		}
	}
	if ! haveContainer {
		errString := fmt.Sprintf("cannot find namespace : %s , pod : %s , containerId :%s", namespace, podName, containerName)
		return nil , errors.New(errString)
	}
	if err = g.Wait(); err != nil{
		return nil , err
	}
	return pids, nil
}

func getPidFromChaosDaemon(ctx context.Context, c client.Client,  pod *v1.Pod , containerId string) (uint32 , error){
	fmt.Printf("getting pid of pod : %s , container : %s from chaos daemon.\n", pod.Name, containerId)
	daemonClient, err := NewChaosDaemonClient(ctx, c , pod, ChaosDaemonPort )
	if err != nil{
		return 0, err
	}
	defer daemonClient.Close()
	if len(pod.Status.ContainerStatuses) == 0 {
		return 0 ,fmt.Errorf("%s %s can't get the state of container", pod.Namespace, pod.Name)
	}

	response , err := daemonClient.ChaosDaemonClient.ContainerGetPid(ctx , &pb.ContainerRequest{
		Action: &pb.ContainerAction{
			Action: pb.ContainerAction_GETPID,
		},
		ContainerId: containerId,
	})
	if err != nil{
		fmt.Printf("Failed to get container pid , namespace : %s, pod : %s , containerId : %s\n", pod.Namespace, pod.Name, containerId)
		return 0 , err
	}

	return response.Pid , nil

}
func usage(){
    fmt.Printf("usage : ./main -namespace namespace -podName podName -containerName containerName\n")
}
func main() {
	//flag.Parse()
	//if podName == "" || containerName == ""{
	//    usage()
	//    return
	//}
	fmt.Printf("http service set up.\n")
	//起一个http服务
    	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    	    	namespace := r.Header.Get("namespace")
    	    	podName   := r.Header.Get("pod")
    	    	containerName := r.Header.Get("container")
    	    	if namespace == ""{
    	    	    namespace = "default"
		}
    	    	if podName == "" || containerName == ""{
    	    	    fmt.Println("the podName or container name are empty!")
    	    	    fmt.Fprintln(w, "the podName or container name are empty!")
		    return
		}

    	    	msg := fmt.Sprintf("getting pid of namespace : %s , pod : %s , container : %s", namespace, podName , containerName)
    	    	fmt.Println(msg)
    	    	fmt.Fprintln(w , msg)

    	    	runtimeClient := NewRuntimeClient()
    	    	ctx := context.Background()
    	    	pids , err := GetTargetPid(ctx , runtimeClient, namespace , podName , containerName)

    	    	if err != nil{
    	    	    	fmt.Println(err)
			fmt.Fprintln(w , err.Error())
		    	return
		}
		msg = fmt.Sprintf("get pid from chaos daemon : \n")
		fmt.Println(msg)
		fmt.Fprintln(w ,msg)
		for index , pid := range pids{
		    msg = fmt.Sprintf("%d : %v\n", index, pid)
		    fmt.Println(msg)
		    fmt.Fprintln(w , msg)
		}
	})
	http.ListenAndServe("0.0.0.0:4567", nil)
}
