#!/usr/bin/env bash
set +e +x
myversion='GO Cross Platform Compilation v2020.1.6'
targets=(aix_ppc64 android_386 android_amd64 android_arm android_arm64 darwin_386 darwin_amd64 darwin_arm darwin_arm64 dragonfly_amd64 freebsd_386 freebsd_amd64 freebsd_arm illumos_amd64 js_wasm linux_386 linux_amd64 linux_arm linux_arm64 linux_ppc64 linux_ppc64le linux_mips linux_mipsle linux_mips64 linux_mips64le linux_s390x netbsd_386 netbsd_amd64 netbsd_arm openbsd_386 openbsd_amd64 openbsd_arm openbsd_arm64 plan9_386 plan9_amd64 plan9_arm solaris_amd64 windows_386 windows_amd64)
appname=$(\basename $(\pwd))
outputpath=bin
packages=''
tags=''
declare -i verbose=0
declare -i nopause=0
declare -i succeeded=0
declare -i failed=0

until [ $# -eq 0 ]; do
  case "$1" in
  --help | -h)
    cat <<EOF
usage: bash ${0##*/} [options] [packages]

packages
  will append to "go build" command

options
  -v, --verbose                print verbose text
  -n, --name <app-name>        the name for format the executable file name
                               like "name_os_arch", default is the directory name
  -o, --output <output-path>   the output path, default is "bin"
  -t, --targets "OS_ARCH ..."  the target OS_ARCH list.
                               like "linux_amd64 darwin_amd64 windows_amd64 windows_386"
                               default is all OS_ARCH
                               from https://golang.org/doc/install/source#environment
      --tags tag,list          the tags for "go build" command
      --no-pause               do not pause when finish
  -h, --help                   display this help and exit
      --version                display this script version and exit
EOF
    exit 0
    ;;
  --verbose | -v)
    verbose=1
    ;;
  --name | -n)
    shift
    if [ -z "$1" ] || [[ "$1" =~ ^\- ]]; then
      printf "ERROR: name argument error: $1\n"
      exit 1
    fi
    appname=$1
    ;;
  --output | -o)
    shift
    if [ -z "$1" ] || [[ "$1" =~ ^\- ]]; then
      printf "ERROR: output argument error: $1\n"
      exit 1
    fi
    outputpath=$1
    ;;
  --targets | -t)
    shift
    if [ -z "$1" ] || [[ "$1" =~ ^\- ]]; then
      printf "ERROR: targets argument error: $1\n"
      exit 1
    fi
    targets=()
    while IFS= read -r -d '' arg; do
      targets+=("$arg")
    done < <(xargs printf '%s\0' <<<"$1")
    ;;
  --tags)
    shift
    if [ -z "$1" ] || [[ "$1" =~ ^\- ]]; then
      printf "ERROR: tags argument error: $1\n"
      exit 1
    fi
    tags="-tags $1"
    ;;
  --no-pause)
    nopause=1
    ;;
  --version)
    printf "$myversion\n"
    exit 1
    ;;
  --)
    shift
    packages="$@"
    break
    ;;
  -)
    printf "ERROR: arguments error. please run \"bash ${0##*/} --help\" to get usage\n"
    exit 1
    ;;
  *)
    packages+=" $1"
    ;;
  esac
  shift
done

if [ ! -z "$packages" ]; then
  declare -i err=0
  while IFS= read -r -d '' arg; do
    if [ ! -e "$arg" ]; then
      let err+=1
      printf "ERROR $err: cannot find the file $arg\n"
    fi
  done < <(xargs printf '%s\0' <<<"$packages")
  if [ $err -gt 0 ]; then
    exit 2
  fi
  # remove left 1 space, this space from first call packages+=" $1"
  packages=${packages:1}
fi

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

cat <<EOF
$myversion
The valid combinations of \$GOOS and \$GOARCH are:
https://golang.org/doc/install/source#environment
https://golang.google.cn/doc/install/source#environment

Packages: $packages
AppName: $appname
Output: $outputpath
Targets: ${targets[@]}
Tags: $tags
EOF

if [ ! -f go.mod ]; then
  printf "\nCreating go.mod ...\n"
  go mod init $appname
  if [ $? -ne 0 ]; then
    printf "Error: $?\n"
    exit 2
  fi
fi

declare -i count=${#targets[*]}
declare -i num=0
for target in "${targets[@]}"; do
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
  env GOOS=$mygoos GOARCH=$mygoarch $(printf "$mycgo") go build $myverbose $tags -ldflags "-s -w" -o $output $packages
  if [ $? -eq 0 ]; then
    let succeeded+=1
    file $output
  else
    let failed+=1
  fi
done

printf "\nDone. $succeeded succeeded, $failed failed, $(ls -1q $outputpath | wc -l | xargs echo) files in $outputpath\n"
if [[ -t 1 && $nopause -eq 0 ]]; then
  printf 'Press any key to exit...'
  read -n1
  printf "\n"
fi
