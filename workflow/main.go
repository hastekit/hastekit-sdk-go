package main

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/pkg/workflow"
)

type Node1 struct {
}

func (n Node1) Type() workflow.NodeType {
	return "Node1"
}

func (n Node1) Validate() error {
	return nil
}

func (n Node1) Execute(ctx context.Context, rs *workflow.Input) (output map[string]any, port string, err error) {
	r := rand.Int()
	fmt.Println("Node1 executing...Generated random number: ", fmt.Sprintf("%d", r))

	return map[string]any{"node1": r}, "default", nil
}

type Node2 struct {
}

func (n Node2) Type() workflow.NodeType {
	return "Node2"
}

func (n Node2) Validate() error {
	return nil
}

func (n Node2) Execute(ctx context.Context, rs *workflow.Input) (output map[string]any, port string, err error) {
	//_, _ = rs.Get("node1")
	fmt.Println("Node2 executing...")

	fmt.Println("Even")

	return map[string]any{"node2": ""}, "default", nil
}

type Node3 struct {
}

func (n Node3) Type() workflow.NodeType {
	return "Node3"
}

func (n Node3) Validate() error {
	return nil
}

func (n Node3) Execute(ctx context.Context, rs *workflow.Input) (output map[string]any, port string, err error) {
	fmt.Println("ODD")
	return map[string]any{"node3": "world"}, "default", nil
}

func main() {
	graph := workflow.NewGraph("sample_workflow")

	graph.AddNode("node1", &Node1{})
	graph.AddNode("node2", &Node2{})
	graph.AddNode("node3", &Node3{})

	graph.AddEdge("START", "node1")

	graph.AddConditionalEdge("node1", func(rs *workflow.Input) string {

		if d, ok := rs.RunContext["node1"]; ok {
			if r, ok := d.(int); ok {
				if r%2 == 0 {
					return "node2"
				}
			}
		}

		return "node3"
	}, map[string]string{"node2": "node2", "node3": "node3"})

	graph.AddEdge("node2", "END")
	graph.AddEdge("node3", "END")

	compiled, err := graph.Compile()
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(compiled)

	results, err := compiled.Execute(context.Background(), &workflow.Input{
		RunID:      uuid.NewString(),
		RunContext: nil,
		Metadata:   nil,
	})
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(results)
}
