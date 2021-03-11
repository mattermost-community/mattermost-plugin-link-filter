# Mattermost Plugin Link Filter

This plugin allows you to filter / surpress links in posts on your Mattermost server. The plugin compares all links in new posts against a configured `Allowed Protocols list`. If the protocol (http, https, s3, etc). is not present in the white list, the post will be removed.

## Installation

1. Go to the [releases page of this Github repository](https://github.com/Brightscout/mattermost-plugin-link-filter/releases) and download the latest release for your Mattermost server.
2. Upload this file in the Mattermost System Console under **System Console > Plugins > Management** to install the plugin. To learn more about how to upload a plugin, [see the documentation](https://docs.mattermost.com/administration/plugins.html#plugin-uploads).
3. Activate the plugin at **System Console > Plugins > Management**.


### Usage

You can edit the plugin configuration in **System Console > Plugins > Embedded Link Filter***
* **Allowed Protocols list**<br>
  This denotes the list of protocols to allow, separated by commas.<br/>
 For example, `http,https` will allow messages with links like `https://github.com` or `http://github.com` but reject posts containing links like `s3://YourS3Bucket/dir/filename.filetype`.

* **New Post Warning Message**<br>
  This denotes the message that is shown when a new post is created and gets rejected.

* **Modified Post Warning Message**<br>
  This denotes the message that is shown when an existing post is modified and gets rejected.

* **Reject Plain Links**<br>
  This is a boolean option. If set, the plugin will also filter posts containing plain text links like `http://www.google.com` in addition to filtering embedded text links.

## License

This repository is under the [Apache 2.0 License](https://github.com/mattermost/mattermost-plugin-link-filter/blob/main/LICENSE).

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FBrightscout%2Fmattermost-plugin-link-filter.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FBrightscout%2Fmattermost-plugin-link-filter?ref=badge_large)


---

Made with &#9829; by [Brightscout](http://www.brightscout.com)
