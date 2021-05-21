module intel/isecl/go-trust-agent/v3

require (
	github.com/google/uuid v1.1.1
	github.com/gorilla/handlers v1.4.2
	github.com/gorilla/mux v1.7.3
	github.com/intel-secl/intel-secl/v3 v3.6.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.6.1
	gopkg.in/yaml.v2 v2.3.0
	intel/isecl/lib/common/v3 v3.6.0
	intel/isecl/lib/platform-info/v3 v3.6.0
	intel/isecl/lib/tpmprovider/v3 v3.6.0
)

replace (
	github.com/intel-secl/intel-secl/v3 => github.com/intel-secl/intel-secl/v3 v3.6.0
	github.com/vmware/govmomi => github.com/arijit8972/govmomi fix-tpm-attestation-output
	intel/isecl/lib/common/v3 => github.com/intel-secl/common/v3 v3.6.0
	intel/isecl/lib/platform-info/v3 => github.com/intel-secl/platform-info/v3 v3.6.0
	intel/isecl/lib/tpmprovider/v3 => github.com/intel-secl/tpm-provider/v3 v3.6.0
)