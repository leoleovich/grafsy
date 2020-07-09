VALID_CATEGORIES+= igports
UID_FILES:=        ${PORTSDIR}/igports/UIDs
GID_FILES:=        ${PORTSDIR}/igports/GIDs
CATEGORIES=        igports

PORTNAME=          grafsy
PORTVERSION=       _PV_
PORTREVISION=      _BN_

MAINTAINER=        kajetan.staszkiewicz@innogames.com
COMMENT=           Graphite client/proxy which listens locally on server and buffers data

USES=              go
BUILD_DEPENDS=     git:devel/git

USE_GITHUB=        nodefault
GH_ACCOUNT=        leoleovich
GH_TUPLE=          leoleovich:grafsy:_GC_:ig_grafsy
GH_TUPLE+=         BurntSushi:toml:master:bs_toml
GH_TUPLE+=         pkg:errors:master:pkg_errors

GRAFSY_USER=       grafsy
GRAFSY_GROUP=      grafsy

USERS=             ${GRAFSY_USER}
GROUPS=            ${GRAFSY_GROUP}

GRAFSY_ETCDIR=     ${PREFIX}/etc/grafsy
GRAFSY_PIDDIR=     /var/run/${PORTNAME}

STRIP=             # stripping can break go binaries

SUB_FILES=         grafsy
SUB_LIST+=         GRAFSY_USER=${GRAFSY_USER} \
                   GRAFSY_GROUP=${GRAFSY_GROUP} \
                   GRAFSY_PIDDIR=${GRAFSY_PIDDIR} \
                   GRAFSY_ETCDIR=${GRAFSY_ETCDIR}

USE_RC_SUBR=       grafsy

PLIST_SUB=         GRAFSY_USER=${GRAFSY_USER} \
                   GRAFSY_GROUP=${GRAFSY_GROUP} \
                   GRAFSY_PIDDIR=${GRAFSY_PIDDIR} \
                   GRAFSY_ETCDIR=${GRAFSY_ETCDIR}

GOPATH=            ${WRKDIR}/gopath
GOSRC=             github.com/${GH_ACCOUNT}/${PORTNAME}
GOCMD=             ${SETENV} ${BUILD_ENV} GOCACHE=/tmp GOPATH=${GOPATH}

post-extract:
	mkdir -p ${WRKDIR}/gopath/src/github.com/${GH_ACCOUNT}
	${MV} ${WRKSRC_ig_grafsy} ${GOPATH}/src/${GOSRC}
	mkdir -p ${WRKDIR}/gopath/src/github.com/BurntSushi
	${MV} ${WRKSRC_bs_toml} ${GOPATH}/src/github.com/BurntSushi/toml
	mkdir -p ${WRKDIR}/gopath/src/github.com/pkg
	${MV} ${WRKSRC_pkg_errors} ${GOPATH}/src/github.com/pkg/errors

do-build:
	${GOCMD} go install ${GOSRC}/${PORTNAME}

do-install:
	${MKDIR} ${STAGEDIR}${GRAFSY_LOGDIR}
	${MKDIR} ${STAGEDIR}${GRAFSY_PIDDIR}
	${MKDIR} ${STAGEDIR}${GRAFSY_ETCDIR}
	${INSTALL_PROGRAM} ${GOPATH}/bin/${PORTNAME} ${STAGEDIR}${PREFIX}/bin/${PORTNAME}
	${INSTALL_PROGRAM} ${GOPATH}/src/${GOSRC}/${PORTNAME}.toml ${STAGEDIR}${GRAFSY_ETCDIR}/${PORTNAME}.toml.sample

.include <bsd.port.options.mk>
.include <bsd.port.mk>
