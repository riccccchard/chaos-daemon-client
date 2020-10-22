## chaos-daemon-client
可以与PingCap的chaos mesh项目中的chaos daemon通信并获取container pid
他会在node上起一个http服务，通过带参数的get访问端口，他会返回并答应container pid

* 构建docker image
```bash
docker build --tag httppidget .
```

*  通过httppidget-deployment.yaml创建pod
```cassandraql
kubectl apply -f httppidget-deployment.yaml
```
他会在pod中起一个http服务，监听node节点的4567端口，如果有http请求，就会调用grpc请求chaos daesmon.

* 通过通过service暴露服务
```cassandraql
kubectl apply -f httppitget-service.yaml
```
他会将集群外的本机的30001端口与集群内node的4567端口绑定，这样就可以用过30001访问http服务了。

*  验证服务
```cassandraql
curl -H "namespace:default" -H "pod:httpapp-68d9c99659-qclmt" -H "container:httpapp" localhost:30001
```
他会去寻找在同一个node上的，namespace 为default，pod名为httpapp-68d9c99659-qclmt , container 名为httpapp
的容器的pid，并返回结果。（请根据情况更改参数）
