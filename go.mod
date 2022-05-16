module intel/isecl/go-trust-agent/v4

require (
	github.com/google/uuid v1.2.0
	github.com/gorilla/handlers v1.4.2
	github.com/gorilla/mux v1.7.3
	github.com/intel-secl/intel-secl/v4 v4.1.2
	github.com/nats-io/nats.go v1.11.1-0.20210623165838-4b75fc59ae30
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	gopkg.in/yaml.v2 v2.4.0
	intel/isecl/lib/common/v4 v4.1.2
	intel/isecl/lib/tpmprovider/v4 v4.1.2
)

replace (
       intel/isecl/lib/common/v4 => github.com/intel-secl/common/v4 v4.1.2
       intel/isecl/lib/tpmprovider/v4 => github.com/intel-secl/tpm-provider/v4 v4.1.2
       github.com/intel-secl/intel-secl/v4 => github.com/intel-secl/intel-secl/v4 v4.1.2
)
