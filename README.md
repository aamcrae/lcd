# LCD
Library to decode 7 segment characters in images.

This library is designed to decode 7 segment digits in images. The decoder operates by configuring
a template that defines the outline of a digit as a quadrilateral, and the width of the segments.
This template is then used to identify one or more 7 segment display digits in an image.

For example, given the image:
![lcd](images/lcd6.jpg)
There are 6 digits in the display that are all the same size, so a digit template can be used
to define the size and shape of the digits, and then a definition of 6 digits that use the template as a base.

The configuration that defines this is:
```
lcd=A,81,0,70,136,-9,135,21,85,127
digit=A,281,253
digit=A,394,251
digit=A,508,252
digit=A,617,252
digit=A,731,251
digit=A,842,252
```
(The configuration uses the [config](http://github.com/aamcrae/config) library to parse
the lines in the file configuration). The core library does not _require_ the use of this config library - the library can be
configured discretely using method calls.

The first line defines a digit template, which is expressed as a name or tag ('A' in this instance),
3 pairs of (X, Y) co-ordinates, a pixel width of the segments
(21 in this case), and an optional (X, Y) co-ordinate defining the centre of the decimal place.
All of the (X, Y) co-ordinates are _offsets_ from a base point (0, 0) that represents the *top left* corner of the digit.
Since the top left corner in the template is always considered (0, 0), it is not included in the template definition.
The first 3 pairs of co-ordinates define the *top right*, *bottom right* and *bottom left* of the outline of the digit.
The final (optional) pair indicates the location of the decimal place. If this pair is not present, no decimal place is
assumed.
The following image shows how the dimensions of the digit are calculated in the template:
![lcd](images/digit.jpg)

After the template, 6 separate lines define the 6 digits in the image, each of which uses the template named 'A' to define the
size of the digit, and a (X, Y) co-ordinate pair that defines the *top left* of the digit.

To make it easy to verify the location of the digits, a utility program named [sample](utils/sample/sample.go) is provided
that reads a configuration and overlays on an image where the digits are e.g run thus:
```
./sample --input=lcd6.jpg --config=lcd6.config --fill=false
```
generates the file output.jpg:
![lcd](images/outline6.jpg)
where each segment's corner has a white arrow placed on it. Running the program with the ```fill``` option turned on:
```
./sample --input=lcd6.jpg --config=lcd6.config
```
generates an image where the actual sampling blocks used to decode the digits are filled in:
![lcd](images/fill6.jpg)

This is not an officially supported Google product.
