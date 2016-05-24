[![GitHub release](https://img.shields.io/github/release/buger/gor.svg?maxAge=3600)](https://github.com/buger/gor/releases) [![codebeat](https://codebeat.co/badges/6427d589-a78e-416c-a546-d299b4089893)](https://codebeat.co/projects/github-com-buger-gor) [![Go Report Card](https://goreportcard.com/badge/github.com/buger/gor)](https://goreportcard.com/report/github.com/buger/gor) [![Join the chat at https://gitter.im/buger/gor](https://badges.gitter.im/buger/gor.svg)](https://gitter.im/buger/gor?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

## About

Gor is an open-source tool for capturing and replaying live HTTP traffic into a test environment in order to continuously test your system with real data. It can be used to increase confidence in code deployments, configuration changes and infrastructure changes.

Now you can test your code on real user sessions in an automated and repeatable fashion.
**No more falling down in production!**

Here is basic workflow: The listener server catches http traffic and sends it to the replay server or saves to file. The replay server forwards traffic to a given address.

![Diagram](http://i.imgur.com/9mqj2SK.png)

## Installation
Download latest binary from https://github.com/buger/gor/releases or (compile by yourself)[Compilation].

## Getting started

The most basic setup will be `sudo ./gor --input-raw :8000 --output-stdout` which acts like tcpdump.
If you already have test environment you can start replaying: `sudo ./gor --input-raw :8000 --output-http http://staging.env`.

See the our wiki and especially [Getting started](https://github.com/buger/gor/wiki/Getting-Started) wiki page for more info. 


## Want to Upgrade?

I also sell Gor Pro, extensions to Gor which provide more features, a commercial-friendly license and allow you to support high quality open source development all at the same time. Please see the Gor [homepage](https://gortool.com/) for more detail.

Subscribe to the [quarterly newsletter](https://tinyletter.com/gor) to stay informed about the latest features and changes to Gor and its bigger siblings.

## Problems?
If you have a problem, please review the [FAQ](https://github.com/buger/gor/wiki/FAQ) and [Troubleshooting](https://github.com/buger/gor/wiki/Troubleshooting) wiki pages. Searching the [issues](https://github.com/buger/gor/issues) for your problem is also a good idea.

All bug-reports and suggestions should go though Github Issues or our [Google Group](https://groups.google.com/forum/#!forum/gor-users) (you can just send email to gor-users@googlegroups.com).
If you have a private question feel free to send email to support@gortool.com.

Useful resources:

* Product documentation is in the [wiki](http://github.com/buger/gor/wiki).
* Release announcements are made to the [@buger](http://twitter.com/buger) Twitter account and our [newsleter](https://tinyletter.com/gor)


If you need commercial support read more about Pro and Enterprise versions at our site [https://gortool.com/](https://gortool.com/)


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

If you are using Gor we are happy add you to the list and share your story, just write to: hello@gortool.com

## Author

Leonid Bugaev, [@buger](https://twitter.com/buger), https://leonsbox.com
