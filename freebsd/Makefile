VALID_CATEGORIES+= igports
UID_FILES:=${PORTSDIR}/igports/UIDs
GID_FILES:=${PORTSDIR}/igports/GIDs
PORTNAME=	grafsy
PORTVERSION=	_PV_
PORTREVISION=	1
CATEGORIES=	igports

MAINTAINER=	kajetan.staszkiewicz@innogames.com
COMMENT=	Graphite client/proxy which listens locally on server and buffers data

BUILD_DEPENDS=	go>=1.5.0:lang/go

CONFLICTS_INSTALL= ig.graphite-client-*

ONLY_FOR_ARCHS=	i386 amd64

USE_GITHUB=	nodefault
GH_TUPLE=	innogames:grafsy:_GC_:grafsy/src/github.com/innogames/grafsy
GH_TUPLE+=	BurntSushi:toml:b26d9c3:toml/src/github.com/BurntSushi/toml
GH_TUPLE+=	innogames:go-acl:c1b74e0:goacl/src/github.com/innogames/go-acl

GRAFSY_USER=	grafsy
GRAFSY_GROUP=	grafsy

USERS=		${GRAFSY_USER}
GROUPS=		${GRAFSY_GROUP}

GRAFSY_ETCDIR=	${PREFIX}/etc/grafsy
GRAFSY_LOGDIR=	/var/log/grafsy
GRAFSY_PIDDIR=	/var/run/${PORTNAME}

STRIP=		# stripping can break go binaries

SUB_FILES=	grafsy
SUB_LIST+=	GRAFSY_USER=${GRAFSY_USER} \
		GRAFSY_GROUP=${GRAFSY_GROUP} \
		GRAFSY_PIDDIR=${GRAFSY_PIDDIR} \
		GRAFSY_LOGDIR=${GRAFSY_LOGDIR} \
		GRAFSY_ETCDIR=${GRAFSY_ETCDIR}

USE_RC_SUBR=	grafsy

PLIST_SUB=	GRAFSY_USER=${GRAFSY_USER} \
		GRAFSY_GROUP=${GRAFSY_GROUP} \
		GRAFSY_PIDDIR=${GRAFSY_PIDDIR} \
		GRAFSY_LOGDIR=${GRAFSY_LOGDIR} \
		GRAFSY_ETCDIR=${GRAFSY_ETCDIR}

do-build:
	cd ${WRKSRC} && ${SETENV} ${BUILD_ENV} GOPATH=${WRKSRC} go build github.com/innogames/grafsy

do-install:
	${MKDIR} ${STAGEDIR}${GRAFSY_LOGDIR}
	${MKDIR} ${STAGEDIR}${GRAFSY_PIDDIR}
	${MKDIR} ${STAGEDIR}${GRAFSY_ETCDIR}
	${INSTALL_PROGRAM} ${WRKSRC}/grafsy ${STAGEDIR}${PREFIX}/bin/grafsy
	${INSTALL_PROGRAM} ${WRKSRC}/src/github.com/innogames/grafsy/grafsy.toml ${STAGEDIR}${GRAFSY_ETCDIR}/grafsy.toml.sample

.include <bsd.port.options.mk>
.include <bsd.port.mk>
