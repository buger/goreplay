[![GitHub release](https://img.shields.io/github/release/buger/gor.svg?maxAge=3600)](https://github.com/buger/goreplay/releases) [![codebeat](https://codebeat.co/badges/6427d589-a78e-416c-a546-d299b4089893)](https://codebeat.co/projects/github-com-buger-gor) [![Go Report Card](https://goreportcard.com/badge/github.com/buger/gor)](https://goreportcard.com/report/github.com/buger/gor) [![Join the chat at https://gitter.im/buger/gor](https://badges.gitter.im/buger/gor.svg)](https://gitter.im/buger/gor?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge) [![Reviewed by Hound](https://img.shields.io/badge/Reviewed_by-Hound-8E64B0.svg)](https://houndci.com)

![Go Replay](http://i.imgur.com/ZG2ki5n.png)

## https://goreplay.org/

## We just released a new version, see announcement https://leonsbox.com/goreplay-v0-16-and-4th-anniversary-5408b1fd72e0

## About

GoReplay is the simplest and safest way to test your app using real traffic before you put it into production.

As your application grows, the effort required to test it also grows exponentially. GoReplay offers you the simple idea of reusing your existing traffic for testing, which makes it incredibly powerful. Our state of art technique allows you to analyze and record your application traffic without affecting it. This eliminates the risks that come with putting a third party component in the critical path. 

GoReplay increases your confidence in code deployments, configuration changes and infrastructure changes. Did we mention that no coding is required?



Here is basic workflow: The listener server catches http traffic and sends it to the replay server or saves to file. The replay server forwards traffic to a given address.

![Diagram](http://i.imgur.com/9mqj2SK.png)

Check [latest documentation](http://github.com/buger/goreplay/wiki).

## Installation
Download latest binary from https://github.com/buger/goreplay/releases or [compile by yourself](https://github.com/buger/goreplay/wiki/Compilation).

## Getting started

The most basic setup will be `sudo ./gor --input-raw :8000 --output-stdout` which acts like tcpdump.
If you already have test environment you can start replaying: `sudo ./gor --input-raw :8000 --output-http http://staging.env`.

See the our [documentation](https://github.com/buger/goreplay/wiki/) and [Getting started](https://github.com/buger/goreplay/wiki/Getting-Started) page for more info. 
## Newsletter
Subscribe to our [newsletter](https://www.getdrip.com/forms/89690474/submissions/new) to stay informed about the latest features and changes to Gor project.


## Want to Upgrade?

We have created a [GoReplay PRO](https://goreplay.org/pro.html) extension which provides additional features such as support for binary protocols like Thrift or ProtocolBuffers, saving and replaying from cloud storage, TCP sessions replication, etc. The PRO version also includes a commercial-friendly license, dedicated support, and it also allows you to support high-quality open source development. 


## Problems?
If you have a problem, please review the [FAQ](https://github.com/buger/goreplay/wiki/FAQ) and [Troubleshooting](https://github.com/buger/goreplay/wiki/Troubleshooting) wiki pages. Searching the [issues](https://github.com/buger/goreplay/issues) for your problem is also a good idea.

All bug-reports and suggestions should go through Github Issues or our [Google Group](https://groups.google.com/forum/#!forum/gor-users) (you can just send email to gor-users@googlegroups.com).
If you have a private question feel free to send email to support@gortool.com.


## Contributing

1. Fork it
2. Create your feature branch (git checkout -b my-new-feature)
3. Commit your changes (git commit -am 'Added some feature')
4. Push to the branch (git push origin my-new-feature)
5. Create new Pull Request

## Companies using Gor

* [GOV.UK](https://www.gov.uk) - UK Government Digital Service
* [theguardian.com](http://theguardian.com) - Most popular online newspaper in the UK
* [TomTom](http://www.tomtom.com/) - Global leader in navigation, traffic and map products, GPS Sport Watches and fleet management solutions.
* [3SCALE](http://www.3scale.net/) - API infrastructure to manage your APIs for internal or external users
* [Optionlab](http://www.opinionlab.com) - Optimize customer experience and drive engagement across multiple channels
* [TubeMogul](http://tubemogul.com) - Software for Brand Advertising
* [Videology](http://www.videologygroup.com/) - Video advertising platform
* [ForeksMobile](http://foreksmobile.com/) -  One of the leading financial application development company in Turkey
* [Granify](http://granify.com) - AI backed SaaS solution that enables online retailers to maximise their sales
* And many more!

If you are using Gor, we are happy to add you to the list and share your story, just write to: hello@goreplay.org

## Author

Leonid Bugaev, [@buger](https://twitter.com/buger), https://leonsbox.com
