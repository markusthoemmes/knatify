package yaml

import (
	"io"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func DecodeInto(reader io.Reader, into interface{}) error {
	decoder := yaml.NewYAMLToJSONDecoder(reader)
	if err := decoder.Decode(into); err != nil {
		return errors.Wrap(err, "failed to decode object")
	}
	return nil
}
