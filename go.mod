module github.com/daptin/daptin

go 1.14

// +heroku goVersion go1.15

require (
	cloud.google.com/go/iam v0.3.0 // indirect
	github.com/GeertJohan/go.rice v1.0.0
	github.com/advance512/yaml v0.0.0-20141213031416-e401b2b02685
	github.com/alecthomas/chroma v0.9.2 // indirect
	github.com/alexeyco/simpletable v0.0.0-20180729223640-1fa9009f1080
	github.com/anthonynsimon/bild v0.10.0
	github.com/araddon/dateparse v0.0.0-20181123171228-21df004e09ca
	github.com/artpar/api2go v2.5.10+incompatible
	github.com/artpar/api2go-adapter v1.0.1
	github.com/artpar/conform v0.0.0-20171227110214-a5409cc587c6
	github.com/artpar/go-guerrilla v1.5.2
	github.com/artpar/go-imap v1.0.3
	github.com/artpar/go-imap-idle v1.0.2
	github.com/artpar/go-smtp-mta v0.2.0
	github.com/artpar/go.uuid v1.2.0
	github.com/artpar/parsemail v0.0.0-20190115161936-abc648830b9a
	github.com/artpar/rclone v1.57.2
	github.com/artpar/resty v1.0.3
	github.com/artpar/stats v1.0.2
	github.com/artpar/xlsx/v2 v2.0.5
	github.com/artpar/ydb v0.1.26
	github.com/aviddiviner/gin-limit v0.0.0-20170918012823-43b5f79762c1
	github.com/bep/golibsass v1.0.0 // indirect
	github.com/bjarneh/latinx v0.0.0-20120329061922-4dfe9ba2a293
	github.com/buraksezer/olric v0.3.0-beta.4
	github.com/corpix/uarand v0.0.0 // indirect
	github.com/disintegration/gift v1.2.1
	github.com/dop251/goja v0.0.0-20181125163413-2dd08a5fc665
	github.com/doug-martin/goqu/v9 v9.11.0
	github.com/emersion/go-message v0.11.1
	github.com/emersion/go-msgauth v0.4.0
	github.com/emersion/go-webdav v0.3.1
	github.com/etgryphon/stringUp v0.0.0-20121020160746-31534ccd8cac // indirect
	github.com/evanw/esbuild v0.12.24 // indirect
	github.com/fclairamb/ftpserver v0.0.0-20200221221851-84e5d668e655
	github.com/frankban/quicktest v1.13.1 // indirect
	github.com/getkin/kin-openapi v0.31.0
	github.com/ghodss/yaml v1.0.0
	github.com/gin-contrib/gzip v0.0.2
	github.com/gin-contrib/static v0.0.0-20181225054800-cf5e10bbd933
	github.com/gin-gonic/gin v1.7.0
	github.com/go-acme/lego/v3 v3.2.0
	github.com/go-gota/gota v0.0.0-20190402185630-1058f871be31
	github.com/go-playground/locales v0.13.0
	github.com/go-playground/universal-translator v0.17.0
	github.com/go-sourcemap/sourcemap v2.1.2+incompatible // indirect
	github.com/go-sql-driver/mysql v1.5.0
	github.com/gobuffalo/envy v1.9.0 // indirect
	github.com/gobuffalo/flect v0.2.3
	github.com/gocarina/gocsv v0.0.0-20181213162136-af1d9380204a
	github.com/gocraft/health v0.0.0-20170925182251-8675af27fef0
	github.com/gohugoio/hugo v0.79.1
	github.com/golang-jwt/jwt/v4 v4.1.0
	github.com/gonum/blas v0.0.0-20181208220705-f22b278b28ac // indirect
	github.com/gonum/floats v0.0.0-20181209220543-c233463c7e82 // indirect
	github.com/gonum/integrate v0.0.0-20181209220457-a422b5c0fdf2 // indirect
	github.com/gonum/internal v0.0.0-20181124074243-f884aa714029 // indirect
	github.com/gonum/lapack v0.0.0-20181123203213-e4cdc5a0bff9 // indirect
	github.com/gonum/matrix v0.0.0-20181209220409-c518dec07be9 // indirect
	github.com/gonum/stat v0.0.0-20181125101827-41a0da705a5b // indirect
	github.com/gopherjs/gopherjs v0.0.0-20190915194858-d3ddacdb130f // indirect
	github.com/gorilla/feeds v1.1.1
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/graphql-go/graphql v0.7.8
	github.com/graphql-go/handler v0.2.3
	github.com/graphql-go/relay v0.0.0-20171208134043-54350098cfe5
	github.com/hashicorp/memberlist v0.1.5
	github.com/iancoleman/strcase v0.0.0-20190422225806-e506e3ef7365
	github.com/icrowley/fake v0.0.0-20180203215853-4178557ae428
	github.com/imroc/req v0.2.4
	github.com/jamiealquiza/envy v1.1.0
	github.com/jdkato/prose v1.2.1 // indirect
	github.com/jinzhu/copier v0.0.0-20180308034124-7e38e58719c3
	github.com/jlaffaye/ftp v0.0.0-20210307004419-5d4190119067
	github.com/jmoiron/sqlx v0.0.0-20181024163419-82935fac6c1a
	github.com/json-iterator/go v1.1.12
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kniren/gota v0.10.1 // indirect
	github.com/labstack/echo v3.3.10+incompatible // indirect
	github.com/labstack/gommon v0.2.8 // indirect
	github.com/lib/pq v1.9.0
	github.com/looplab/fsm v0.0.0-20180515091235-f980bdb68a89
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/naoina/toml v0.1.1
	github.com/niklasfasching/go-org v1.5.0 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/pelletier/go-toml v1.9.3 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pquerna/otp v1.2.0
	github.com/robfig/cron/v3 v3.0.0
	github.com/sadlil/go-trigger v0.0.0-20170328161825-cfc3d83007cd
	github.com/sanity-io/litter v1.5.1 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/siebenmann/smtpd v0.0.0-20170816215504-b93303610bbe // indirect
	github.com/simplereach/timeutils v1.2.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/smancke/mailck v0.0.0-20180319162224-be54df53c96e
	github.com/smartystreets/assertions v1.0.0 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/cobra v1.4.0
	github.com/tdewolff/minify/v2 v2.9.21 // indirect
	github.com/yangxikun/gin-limit-by-key v0.0.0-20190512072151-520697354d5f
	github.com/yuin/goldmark v1.4.0 // indirect
	gocloud.dev v0.22.0 // indirect
	golang.org/x/crypto v0.0.0-20220331220935-ae2d96664a29
	golang.org/x/image v0.0.0-20210220032944-ac19c3e999fb // indirect
	golang.org/x/net v0.0.0-20220325170049-de3da57026de
	golang.org/x/oauth2 v0.0.0-20220309155454-6242fa91716a
	golang.org/x/text v0.3.7
	golang.org/x/time v0.0.0-20220224211638-0e9765cccd65
	gonum.org/v1/gonum v0.6.2 // indirect
	gopkg.in/go-playground/validator.v9 v9.30.0
	gopkg.in/ini.v1 v1.62.0 // indirect
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.0.0+incompatible

//replace github.com/daptin/daptin v0.9.6 => github.com/Ghvstcode/daptin v0.9.6
