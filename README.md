# go-libp2p-netsync

Is a distributed locking service built on top of the Libp2p networking stack. It facilitates efficient, peer-to-peer distributed
locks to coordinate resource access in decentralized systems.The service uses the Libp2p DHT for peer discovery and communication and
Protobuf for lightweight and efficient message serialization. It wont provide 100% network lock can be used to access resources that cant handle 
more concurrent requests.
