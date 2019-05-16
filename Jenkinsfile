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
            stage('Build, tagging and publishing from branch') {
                if ( env.GIT_BRANCH_OR_TAG == 'branch' && jobCommon.launchedByUser() ) {
                    sshagent (['jenkins-rsa']) {
                        sh '''\
                            #!/bin/bash -ex
                            gbp dch --git-author -D stretch --ignore-branch --commit -N "${NEW_VERSION}" -a
                            git tag "${GIT_NEW_TAG}"
                            git tag "${NEW_VERSION}"

                            debuild -b -us -uc
                            '''.stripIndent()

                        if ( ! sh(script: "git remote show ${GIT_REMOTE}", returnStdout: true).contains("HEAD branch: ${env.GIT_LOCAL_BRANCH}") ) {
                            echo 'Pushing changes bask is disabled for non default branches'
                            return
                        }

                        if (env.REPO_NAME) {
                            withCredentials([string(credentialsId: 'DEB_DROP_TOKEN', variable: 'DebDropToken')]) {
                                jobCommon.uploadPackage  file: "${env.WORKSPACE}/${env.PACKAGE_NAME}_${env.NEW_VERSION}_amd64.deb", repo: env.REPO_NAME, token: DebDropToken
                                jobCommon.uploadPackage  file: "${env.WORKSPACE}/${env.PACKAGE_NAME}-dbgsym_${env.NEW_VERSION}_amd64.deb", repo: env.REPO_NAME, token: DebDropToken
                            }
                        }

                        // We push changes back to git only after successful package uploading
                        sh '''\
                            #!/bin/bash -ex

                            git push ${GIT_REMOTE} HEAD:${GIT_LOCAL_BRANCH}
                            git push --tags ${GIT_REMOTE}
                            '''.stripIndent()
                    }
                }
            }
            stage('Building from tag') {
                if ( env.GIT_BRANCH_OR_TAG == 'tag' && jobCommon.launchedByUser() ) {
                    sshagent (['jenkins-rsa']) {
                        sh '''\
                            #!/bin/bash -ex
                            debuild -b -us -uc
                            '''.stripIndent()

                        if (env.REPO_NAME) {
                            withCredentials([string(credentialsId: 'DEB_DROP_TOKEN', variable: 'DebDropToken')]) {
                                jobCommon.uploadPackage  file: "${env.WORKSPACE}/${env.PACKAGE_NAME}_${env.NEW_VERSION}_amd64.deb", repo: env.REPO_NAME, token: DebDropToken
                                jobCommon.uploadPackage  file: "${env.WORKSPACE}/${env.PACKAGE_NAME}-dbgsym_${env.NEW_VERSION}_amd64.deb", repo: env.REPO_NAME, token: DebDropToken
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
