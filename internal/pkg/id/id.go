package id

import "github.com/bwmarrin/snowflake"

var node *snowflake.Node

// Init initializes the snowflake node with the given node ID.
func Init(nodeID int64) error {
	n, err := snowflake.NewNode(nodeID)
	if err != nil {
		return err
	}
	node = n
	return nil
}

// New returns a new unique string ID.
func New() string {
	return node.Generate().String()
}
