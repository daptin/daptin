module github.com/daptin/daptin

go 1.14

// +heroku goVersion go1.15

require (
	cloud.google.com/go/iam v0.3.0 // indirect
	github.com/Azure/azure-amqp-common-go v1.1.4 // indirect
	github.com/GeertJohan/go.rice v1.0.0
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/advance512/yaml v0.0.0-20141213031416-e401b2b02685
	github.com/alexeyco/simpletable v0.0.0-20180729223640-1fa9009f1080
	github.com/anthonynsimon/bild v0.10.0
	github.com/araddon/dateparse v0.0.0-20181123171228-21df004e09ca
	github.com/armon/consul-api v0.0.0-20180202201655-eb2c6b5be1b6 // indirect
	github.com/artpar/api2go v2.5.10+incompatible
	github.com/artpar/api2go-adapter v1.0.1
	github.com/artpar/conform v0.0.0-20171227110214-a5409cc587c6
	github.com/artpar/go-guerrilla v1.5.2
	github.com/artpar/go-httpclient v1.0.0 // indirect
	github.com/artpar/go-imap v1.0.3
	github.com/artpar/go-imap-idle v1.0.2
	github.com/artpar/go-koofrclient v1.0.1 // indirect
	github.com/artpar/go-smtp-mta v0.2.0
	github.com/artpar/go.uuid v1.2.0
	github.com/artpar/parsemail v0.0.0-20190115161936-abc648830b9a
	github.com/artpar/rclone v1.57.2
	github.com/artpar/resty v1.0.3
	github.com/artpar/stats v1.0.2
	github.com/artpar/xlsx/v2 v2.0.5
	github.com/artpar/ydb v0.1.26
	github.com/aviddiviner/gin-limit v0.0.0-20170918012823-43b5f79762c1
	github.com/beevik/etree v1.1.0 // indirect
	github.com/bjarneh/latinx v0.0.0-20120329061922-4dfe9ba2a293
	github.com/bradfitz/iter v0.0.0-20190303215204-33e6a9893b0c // indirect
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce // indirect
	github.com/buraksezer/olric v0.3.6
	github.com/corpix/uarand v0.0.0 // indirect
	github.com/disintegration/gift v1.2.1
	github.com/dop251/goja v0.0.0-20181125163413-2dd08a5fc665
	github.com/doug-martin/goqu/v9 v9.11.0
	github.com/dropbox/dropbox-sdk-go-unofficial v5.6.0+incompatible // indirect
	github.com/emersion/go-message v0.11.1
	github.com/emersion/go-msgauth v0.4.0
	github.com/emersion/go-webdav v0.3.1
	github.com/etgryphon/stringUp v0.0.0-20121020160746-31534ccd8cac // indirect
	github.com/fclairamb/ftpserver v0.0.0-20200221221851-84e5d668e655
	github.com/form3tech-oss/jwt-go v3.2.5+incompatible // indirect
	github.com/getkin/kin-openapi v0.75.0
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
	github.com/gohugoio/hugo v0.88.1
	github.com/golang-jwt/jwt/v4 v4.1.0
	github.com/gonum/blas v0.0.0-20181208220705-f22b278b28ac // indirect
	github.com/gonum/floats v0.0.0-20181209220543-c233463c7e82 // indirect
	github.com/gonum/integrate v0.0.0-20181209220457-a422b5c0fdf2 // indirect
	github.com/gonum/internal v0.0.0-20181124074243-f884aa714029 // indirect
	github.com/gonum/lapack v0.0.0-20181123203213-e4cdc5a0bff9 // indirect
	github.com/gonum/matrix v0.0.0-20181209220409-c518dec07be9 // indirect
	github.com/gonum/stat v0.0.0-20181125101827-41a0da705a5b // indirect
	github.com/gopherjs/gopherjs v0.0.0-20190915194858-d3ddacdb130f // indirect
	github.com/goreleaser/goreleaser v0.162.0 // indirect
	github.com/gorilla/feeds v1.1.1
	github.com/graphql-go/graphql v0.7.8
	github.com/graphql-go/handler v0.2.3
	github.com/graphql-go/relay v0.0.0-20171208134043-54350098cfe5
	github.com/hashicorp/memberlist v0.1.5
	github.com/iancoleman/strcase v0.0.0-20190422225806-e506e3ef7365
	github.com/icrowley/fake v0.0.0-20180203215853-4178557ae428
	github.com/imroc/req v0.2.4
	github.com/jamiealquiza/envy v1.1.0
	github.com/jinzhu/copier v0.0.0-20180308034124-7e38e58719c3
	github.com/jlaffaye/ftp v0.0.0-20210307004419-5d4190119067
	github.com/jmoiron/sqlx v0.0.0-20181024163419-82935fac6c1a
	github.com/json-iterator/go v1.1.12
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kniren/gota v0.10.1 // indirect
	github.com/koofr/go-httpclient v0.0.0-20200420163713-93aa7c75b348 // indirect
	github.com/koofr/go-koofrclient v0.0.0-20190724113126-8e5366da203a // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/kyokomi/emoji v2.2.4+incompatible // indirect
	github.com/labstack/echo v3.3.10+incompatible // indirect
	github.com/labstack/gommon v0.2.8 // indirect
	github.com/laurent22/ical-go v0.1.0 // indirect
	github.com/lib/pq v1.9.0
	github.com/looplab/fsm v0.0.0-20180515091235-f980bdb68a89
	github.com/markbates/inflect v1.0.4 // indirect
	github.com/marten-seemann/qtls-go1-15 v0.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/mitchellh/gox v1.0.1 // indirect
	github.com/naoina/toml v0.1.1
	github.com/ncw/swift v1.0.52 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.1.1 // indirect
	github.com/okzk/sdnotify v0.0.0-20180710141335-d9becc38acbd // indirect
	github.com/pkg/errors v0.9.1
	github.com/pquerna/otp v1.2.0
	github.com/rclone/rclone v1.58.0 // indirect
	github.com/robfig/cron/v3 v3.0.0
	github.com/sadlil/go-trigger v0.0.0-20170328161825-cfc3d83007cd
	github.com/samedi/caldav-go v3.0.0+incompatible // indirect
	github.com/sevlyar/go-daemon v0.1.5 // indirect
	github.com/siebenmann/smtpd v0.0.0-20170816215504-b93303610bbe // indirect
	github.com/simplereach/timeutils v1.2.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/smancke/mailck v0.0.0-20180319162224-be54df53c96e
	github.com/spf13/cobra v1.4.0
	github.com/tidwall/pretty v0.0.0-20190325153808-1166b9ac2b65 // indirect
	github.com/uber/jaeger-client-go v2.15.0+incompatible // indirect
	github.com/uber/jaeger-lib v1.5.0 // indirect
	github.com/urfave/cli v1.22.4 // indirect
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c // indirect
	github.com/xdg/stringprep v1.0.0 // indirect
	github.com/xordataexchange/crypt v0.0.3-0.20170626215501-b2862e3d0a77 // indirect
	github.com/yangxikun/gin-limit-by-key v0.0.0-20190512072151-520697354d5f
	go.mongodb.org/mongo-driver v1.0.1 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.0 // indirect
	golang.org/x/crypto v0.0.0-20220331220935-ae2d96664a29
	golang.org/x/net v0.0.0-20220325170049-de3da57026de
	golang.org/x/oauth2 v0.0.0-20220309155454-6242fa91716a
	golang.org/x/text v0.3.7
	golang.org/x/time v0.0.0-20220224211638-0e9765cccd65
	gonum.org/v1/gonum v0.6.2 // indirect
	gopkg.in/go-playground/validator.v9 v9.30.0
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	pack.ag/amqp v0.11.0 // indirect
)

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.0.0+incompatible

//replace github.com/daptin/daptin v0.9.6 => github.com/Ghvstcode/daptin v0.9.6
