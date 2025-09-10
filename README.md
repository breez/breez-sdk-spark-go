# Breez SDK - Nodeless (*Spark Implementation*)

## **What Is the Breez SDK?**

The Breez SDK provides developers with an end-to-end solution for integrating self-custodial Lightning into their apps and services. It eliminates the need for third parties, simplifies the complexities of Bitcoin and Lightning, and enables seamless onboarding for billions of users to the future of value transfer.

## **What Is the Breez SDK - Nodeless *(Spark Implementation)*?**

It’s a nodeless integration that offers a self-custodial, end-to-end solution for integrating Lightning payments, utilizing Spark with on-chain interoperability and third-party fiat on-ramps.

## Installation

To install the package:

```sh
$ go get github.com/breez/breez-sdk-spark-go
```

### Supported platforms

This package embeds the Breez SDK - Nodeless *(Spark Implementation)* runtime compiled as shared library objects, and uses [`cgo`](https://golang.org/cmd/cgo/) to consume it. A set of precompiled shared library objects are provided. Thus this package works (and is tested) on the following platforms:

<table>
  <thead>
    <tr>
      <th>Platform</th>
      <th>Architecture</th>
      <th>Triple</th>
      <th>Status</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td rowspan="2">Android</td>
      <td><code>amd64</code></td>
      <td><code>x86_64-linux-android</code></td>
      <td>✅</td>
    </tr>
    <tr>
      <td><code>aarch64</code></td>
      <td><code>aarch64-linux-android</code></td>
      <td>✅</td>
    </tr>
    <tr>
      <td rowspan="2">Darwin</td>
      <td><code>amd64</code></td>
      <td><code>x86_64-apple-darwin</code></td>
      <td>✅</td>
    </tr>
    <tr>
      <td><code>aarch64</code></td>
      <td><code>aarch64-apple-darwin</code></td>
      <td>✅</td>
    </tr>
    <tr>
      <td rowspan="2">Linux</td>
      <td><code>amd64</code></td>
      <td><code>x86_64-unknown-linux-gnu</code></td>
      <td>✅</td>
    </tr>
    <tr>
      <td><code>aarch64</code></td>
      <td><code>aarch64-unknown-linux-gnu</code></td>
      <td>✅</td>
    </tr>
    <tr>
      <td>Windows</td>
      <td><code>amd64</code></td>
      <td><code>x86_64-pc-windows-msvc</code></td>
      <td>✅</td>
    </tr>
  </tbody>
</table>

## Usage

Head over to the Breez SDK - Nodeless *(Spark Implementation)* [documentation](https://sdk-doc-spark.breez.technology/) to start implementing Lightning in your app.

```go
package main

import (
	"github.com/breez/breez-sdk-spark-go/breez_sdk_spark"
)

func main() {
    mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

    config := breez_sdk_spark.DefaultConfig(breez_sdk_spark.NetworkMainnet)

    sdk, err := breez_sdk_spark.Connect(breez_sdk_spark.ConnectRequest{
        Config:     config,
        Mnemonic:   mnemonic,
        StorageDir: "./.data",
    })
}
```

## Bundling

For some platforms the provided binding libraries need to be copied into a location where they need to be found during runtime.

### Android

Copy the binding libraries into the jniLibs directory of your app
```bash
cp vendor/github.com/breez/breez-sdk-spark-go/breez_sdk_spark/lib/android-aarch64/*.so android/app/src/main/jniLibs/arm64-v8a/
cp vendor/github.com/breez/breez-sdk-spark-go/breez_sdk_spark/lib/android-amd64/*.so android/app/src/main/jniLibs/x86_64/
```
So they are in the following structure
```
└── android
    ├── app
        └── src
            └── main
                └── jniLibs
                    ├── arm64-v8a
                        ├── libbreez_sdk_spark_bindings.so
                        └── libc++_shared.so
                    └── x86_64
                        ├── libbreez_sdk_spark_bindings.so
                        └── libc++_shared.so
                └── AndroidManifest.xml
        └── build.gradle
    └── build.gradle
```

### Windows

Copy the binding library to the same directory as the executable file or include the library into the windows install packager.
```bash
cp vendor/github.com/breez/breez-sdk-spark-go/breez_sdk_spark/lib/windows-amd64/*.dll build/windows/
```

## Pricing

The Breez SDK is **free** for developers. 

## Support

Have a question for the team? Join us on [Telegram](https://t.me/breezsdk) or email us at <contact@breez.technology>.

## Information for Maintainers and Contributors

This repository is used to publish a Go package providing Go bindings to the Breez SDK - Nodeless *(Spark Implementation)*'s [underlying Rust implementation](https://github.com/breez/spark-sdk). The Go bindings are generated using [UniFFi Bindgen Go](https://github.com/NordSecurity/uniffi-bindgen-go).

Any changes to Breez SDK - Nodeless *(Spark Implementation)*, the Go bindings, and the configuration of this Go package must be made via the [spark-sdk](https://github.com/breez/spark-sdk) repository.
