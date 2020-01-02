#!/usr/bin/env bash
set +e +x
cat << EOF
GO Cross Platform Compilation
The valid combinations of \$GOOS and \$GOARCH are:
https://golang.org/doc/install/source#environment
https://golang.google.cn/doc/install/source#environment

EOF

GOOS_GOARCH_array=( aix_ppc64 android_386 android_amd64 android_arm android_arm64 darwin_386 darwin_amd64 darwin_arm darwin_arm64 dragonfly_amd64 freebsd_386 freebsd_amd64 freebsd_arm illumos_amd64 js_wasm linux_386 linux_amd64 linux_arm linux_arm64 linux_ppc64 linux_ppc64le linux_mips linux_mipsle linux_mips64 linux_mips64le linux_s390x netbsd_386 netbsd_amd64 netbsd_arm openbsd_386 openbsd_amd64 openbsd_arm openbsd_arm64 plan9_386 plan9_amd64 plan9_arm solaris_amd64 windows_386 windows_amd64 )
appname=`\basename $(\pwd)`
outputpath=./bin
package=''
declare -i verbose=0

until [ $# -eq 0 ]; do
    case "$1" in
        --help|-h)
            printf "usage: bash ${0##*/} [--verbose|-v] [--package|-p <package-name>] [--name|-n <app-name>] [--output|-o <output-path>]\n"
            exit 0
        ;;
        --verbose|-v)
            verbose=1
        ;;
        --package|-n)
            shift
            if [ -z "$1" ] || [[ "$1" =~ ^\- ]]; then
                printf "ERROR: package argument error: $1\n"
                exit 1
            fi
            package=$1
        ;;
        --name|-n)
            shift
            if [ -z "$1" ] || [[ "$1" =~ ^\- ]]; then
                printf "ERROR: name argument error: $1\n"
                exit 1
            fi
            appname=$1
        ;;
        --output|-o)
            shift
            if [ -z "$1" ] || [[ "$1" =~ ^\- ]]; then
                printf "ERROR: output argument error: $1\n"
                exit 1
            fi
            outputpath=$1
        ;;
        *)
            printf "ERROR: arguments error. please run \"bash ${0##*/} --help\" to get usage\n"
            exit 1
        ;;
    esac
    shift
done

if [ -z "$appname" ]; then
    printf "ERROR: please input an app-name by --name\n"
    exit 2
fi

if [ -z "$outputpath" ]; then
    printf "ERROR: please input an output-path by --output\n"
    exit 2
fi

mkdir -p "$outputpath"
if [ $? -ne 0 ]; then
    printf "ERROR: create $outputpath failed: $?\n"
    exit 2
fi

cat << EOF
Package: $package
AppName: $appname
Output: $outputpath
EOF

if [ ! -f go.mod ]; then
    printf "\nCreating go.mod ...\n"
    go mod init $appname
    if [ $? -ne 0 ]; then
        printf "Error: $?\n"
        exit 2
    fi
fi

declare -i count=${#GOOS_GOARCH_array[*]}
declare -i num=0
for target in "${GOOS_GOARCH_array[@]}"; do
    let num+=1
    output=$outputpath/${appname}_${target}
    mygoos=${target%%_*}
    mygoarch=${target##*_}
    myverbose=''
    mycgo=''
    if [ $verbose -eq 1 ]; then
        myverbose='-v'
    fi
    case "$mygoos" in
        windows)
            output="$output.exe"
        ;;
        android)
            mycgo='CGO_ENABLED=1'
        ;;
    esac
    printf "\n($num/$count) Building $output ...\n"
    env GOOS=$mygoos GOARCH=$mygoarch `printf "$mycgo"` go build $myverbose -ldflags "-s -w" -o $output $package
    file $output
done

if [ -t 1 ]; then
    printf "\nDone. Press any key to exit..."
    read -n1
    printf "\n"
fi
