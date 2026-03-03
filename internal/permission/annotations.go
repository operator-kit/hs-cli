package permission

import "github.com/spf13/cobra"

// Annotation keys stored on cobra.Command.Annotations.
const (
	AnnotationResource  = "hs:resource"
	AnnotationOperation = "hs:operation"
)

// Annotate sets the resource and operation annotations on a command.
func Annotate(cmd *cobra.Command, resource, operation string) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[AnnotationResource] = resource
	cmd.Annotations[AnnotationOperation] = operation
}
