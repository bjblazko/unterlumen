Unterlumen container image — licenses and sources
==================================================

Unterlumen (/unterlumen) is licensed under the Apache License 2.0; the text
is in LICENSE next to this file. Its source is at
https://github.com/bjblazko/unterlumen

The libraries, typefaces and scripts built into Unterlumen keep their own
licenses. Their texts are in licenses/ next to this file, and the app lists
them under About > Licenses and thanks (/#licenses).

The helper programs in this image -- FFmpeg, ExifTool, libheif
(heif-convert), libde265 and libwebp (cwebp) -- and the rest of the system
are Debian packages, unchanged. They are not part of Unterlumen and keep
their own licenses, some of them GPL or LGPL:

  - each package's license:  /usr/share/doc/<package>/copyright
  - installed versions:       dpkg-query -W
  - their source code:        https://sources.debian.org/src/<package>/<version>/
                              or `apt-get source <package>=<version>`
                              (Debian keeps every released version at
                              https://snapshot.debian.org)
