module github.com/daptin/daptin

require (
	github.com/GeertJohan/go.rice v1.0.0
	github.com/Masterminds/squirrel v1.1.0
	github.com/PuerkitoBio/goquery v1.5.0
	github.com/RoaringBitmap/roaring v0.4.21 // indirect
	github.com/advance512/yaml v0.0.0-20141213031416-e401b2b02685
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/alexeyco/simpletable v0.0.0-20180729223640-1fa9009f1080
	github.com/anacrolix/envpprof v1.1.0 // indirect
	github.com/anacrolix/tagflag v1.0.1 // indirect
	github.com/anthonynsimon/bild v0.10.0
	github.com/araddon/dateparse v0.0.0-20181123171228-21df004e09ca
	github.com/artpar/api2go v2.4.0+incompatible
	github.com/artpar/api2go-adapter v1.0.0
	github.com/artpar/conform v0.0.0-20171227110214-a5409cc587c6
	github.com/artpar/go-guerrilla v1.5.2
	github.com/artpar/go-imap v1.0.3
	github.com/artpar/go-imap-idle v1.0.2
	github.com/artpar/go-smtp-mta v0.2.0
	github.com/artpar/go.uuid v1.2.0
	github.com/artpar/parsemail v0.0.0-20190115161936-abc648830b9a
	github.com/artpar/quickgomail v0.3.0
	github.com/artpar/rclone v1.50.3
	github.com/artpar/resty v1.0.1
	github.com/artpar/stats v1.0.2
	github.com/aws/aws-sdk-go v1.25.31
	github.com/bamzi/jobrunner v0.0.0-20161019143021-273175f8b6eb // indirect
	github.com/bjarneh/latinx v0.0.0-20120329061922-4dfe9ba2a293
	github.com/coreos/etcd v3.3.15+incompatible // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/corpix/uarand v0.0.0 // indirect
	github.com/creack/pty v1.1.9 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/digitalocean/godo v1.1.3 // indirect
	github.com/disintegration/gift v1.2.0
	github.com/dlclark/regexp2 v0.0.0-20171009020623-7632a260cbaf // indirect
	github.com/dop251/goja v0.0.0-20181125163413-2dd08a5fc665
	github.com/emersion/go-message v0.11.1
	github.com/emersion/go-msgauth v0.4.0
	github.com/emersion/go-sasl v0.0.0-20191210011802-430746ea8b9b
	github.com/emersion/go-smtp-mta v0.0.0-20170206201558-f9b2f2fd6e9a
	github.com/etgryphon/stringUp v0.0.0-20121020160746-31534ccd8cac // indirect
	github.com/gabriel-vasile/mimetype v0.3.15 // indirect
	github.com/getkin/kin-openapi v0.2.0
	github.com/ghodss/yaml v1.0.0
	github.com/gin-contrib/sse v0.0.0-20190301062529-5545eab6dad3 // indirect
	github.com/gin-contrib/static v0.0.0-20181225054800-cf5e10bbd933
	github.com/gin-gonic/gin v1.3.0
	github.com/glycerine/go-unsnap-stream v0.0.0-20190901134440-81cf024a9e0a // indirect
	github.com/go-acme/lego/v3 v3.2.0
	github.com/go-gota/gota v0.0.0-20190402185630-1058f871be31
	github.com/go-playground/locales v0.12.1
	github.com/go-playground/universal-translator v0.16.0
	github.com/go-playground/validator v9.29.1+incompatible // indirect
	github.com/go-sourcemap/sourcemap v2.1.2+incompatible // indirect
	github.com/go-sql-driver/mysql v1.4.1
	github.com/gobuffalo/flect v0.1.5
	github.com/gocarina/gocsv v0.0.0-20181213162136-af1d9380204a
	github.com/gocraft/health v0.0.0-20170925182251-8675af27fef0
	github.com/gogo/protobuf v1.3.0 // indirect
	github.com/gonum/blas v0.0.0-20181208220705-f22b278b28ac // indirect
	github.com/gonum/floats v0.0.0-20181209220543-c233463c7e82 // indirect
	github.com/gonum/internal v0.0.0-20181124074243-f884aa714029 // indirect
	github.com/gonum/lapack v0.0.0-20181123203213-e4cdc5a0bff9 // indirect
	github.com/gonum/matrix v0.0.0-20181209220409-c518dec07be9 // indirect
	github.com/gonum/stat v0.0.0-20181125101827-41a0da705a5b // indirect
	github.com/google/pprof v0.0.0-20190908185732-236ed259b199 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20190915194858-d3ddacdb130f // indirect
	github.com/gorilla/feeds v1.1.1
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/graphql-go/graphql v0.7.8
	github.com/graphql-go/handler v0.2.3
	github.com/graphql-go/relay v0.0.0-20171208134043-54350098cfe5
	github.com/h2non/filetype v1.0.9
	github.com/hpcloud/tail v1.0.0
	github.com/iancoleman/strcase v0.0.0-20190422225806-e506e3ef7365
	github.com/icrowley/fake v0.0.0-20180203215853-4178557ae428
	github.com/imroc/req v0.2.4
	github.com/jamiealquiza/envy v1.1.0
	github.com/jinzhu/copier v0.0.0-20180308034124-7e38e58719c3
	github.com/jmoiron/sqlx v0.0.0-20181024163419-82935fac6c1a
	github.com/json-iterator/go v1.1.7
	github.com/julienschmidt/httprouter v1.2.0
	github.com/kr/pty v1.1.8 // indirect
	github.com/labstack/echo v0.0.0-20181205161348-3f8b45c8d0dd // indirect
	github.com/leodido/go-urn v0.0.0-20181204092800-a67a23e1c1af // indirect
	github.com/lib/pq v1.0.0
	github.com/looplab/fsm v0.0.0-20180515091235-f980bdb68a89
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mattn/go-pointer v0.0.0-20190911064623-a0a44394634f // indirect
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/mattn/goveralls v0.0.2 // indirect
	github.com/mholt/certmagic v0.6.1 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/onsi/ginkgo v1.10.1 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/pquerna/otp v1.2.0
	github.com/prometheus/common v0.7.0 // indirect
	github.com/prometheus/procfs v0.0.5 // indirect
	github.com/robfig/cron v1.0.0
	github.com/rogpeppe/fastuuid v1.2.0 // indirect
	github.com/rogpeppe/go-internal v1.4.0 // indirect
	github.com/russross/blackfriday v2.0.0+incompatible
	github.com/sadlil/go-trigger v0.0.0-20170328161825-cfc3d83007cd
	github.com/sirupsen/logrus v1.4.2
	github.com/smancke/mailck v0.0.0-20180319162224-be54df53c96e
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.4.0
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/tealeg/xlsx v0.0.0-20181024002044-dbf71b6a931e
	github.com/ugorji/go v1.1.7 // indirect
	golang.org/x/crypto v0.0.0-20191108234033-bd318be0434a
	golang.org/x/image v0.0.0-20190910094157-69e4b8554b2a // indirect
	golang.org/x/mobile v0.0.0-20190923204409-d3ece3b6da5f // indirect
	golang.org/x/net v0.0.0-20191109021931-daa7c04131f5
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/text v0.3.2
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/go-playground/validator.v9 v9.29.1
	gopkg.in/src-d/go-billy.v4 v4.3.0 // indirect
	gopkg.in/src-d/go-git.v4 v4.8.1
)

go 1.13
