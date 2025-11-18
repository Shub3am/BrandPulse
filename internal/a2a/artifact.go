// artifact.go builds the one reply envelope every agent returns.
//
// It is a separate file from serve.go because the envelope is the contract
// DronaHQ binds to, and it changes for different reasons than the server
// wiring does.
//
// STUB: signature only, body panics. B1 Task 11 implements this.
package a2a

// JSONArtifact marshals v into the single application/json artifact an agent
// replies with. name is the output type name, such as "MentionBatch", which is
// what DronaHQ keys its bindings on.
func JSONArtifact(name string, v any) (Artifact, error) {
	panic("not implemented")
}
