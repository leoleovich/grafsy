#!/usr/bin/groovy
library 'adminsLib@master'
properties([
    parameters([
        choice(choices: ['patch', 'minor', 'major'].join('\n'), description: 'Which part of version should be updated', name: 'UPDATE_VERSION'),
        string(defaultValue: '', description: 'deb-drop repository', name: 'REPO_NAME', trim: false),
    ])
])

// Remove builds in presented status, default is ['ABORTED', 'NOT_BUILT']
jobCommon.cleanNotFinishedBuilds()

node('docker') {
    // Checkout repo and get info about current stage
    sh 'echo Initial env; env | sort'
    env.PACKAGE_NAME = 'grafsy'
    docker.image('felixoid/grafsy-builder:latest').pull()
    docker.image('felixoid/grafsy-builder:latest').inside("${jobCommon.dockerArgs()}") {
    ansiColor('xterm') {
    try {
        stage('Checkout') {
            env.GIT_CHECKOUT_DIR = 'git'
            gitSteps checkout: true, checkout_extensions: [$class: 'RelativeTargetDirectory', relativeTargetDir: env.GIT_CHECKOUT_DIR]
            sh 'set +x; echo "Environment variables after checkout:"; env|sort'
            if (! jobCommon.launchedByUser()) {
                currentBuild.result = 'NOT_BUILT'
                error 'Automatically triggered build with only jenkins release commits'
            }
        }
        dir(env.GIT_CHECKOUT_DIR) {
            stage('Building') {
                if ( jobCommon.launchedByUser() ) {
                    sshagent (['jenkins-rsa']) {
                        sh '''\
                            #!/bin/bash -ex
                            debuild -b -us -uc
                            '''.stripIndent()

                        if (env.REPO_NAME) {
                            env.PACKAGE_VERSION = sh(returnStdout: true, script: 'dpkg-parsechangelog -l debian/changelog -S Version').trim()
                            withCredentials([string(credentialsId: 'DEB_DROP_TOKEN', variable: 'DebDropToken')]) {
                                jobCommon.uploadPackage  file: "${env.WORKSPACE}/${env.PACKAGE_NAME}_${env.PACKAGE_VERSION}_amd64.deb", repo: env.REPO_NAME, token: DebDropToken
                                jobCommon.uploadPackage  file: "${env.WORKSPACE}/${env.PACKAGE_NAME}-dbgsym_${env.PACKAGE_VERSION}_amd64.deb", repo: env.REPO_NAME, token: DebDropToken
                            }
                        }
                    }
                }
            }
            // TODO: make a release with deb files
        }
        cleanWs(notFailBuild: true)
    }
    catch (all) {
        currentBuild.result = 'FAILURE'
        error "Something wrong, exception is: ${all}"
        jobCommon.processException(all)
    }
    finally {
        jobCommon.postSlack()
    }
    }
    }
}
