package de.huepattl.unterlumen.photo

import com.drew.imaging.ImageMetadataReader
import com.drew.metadata.Directory
import com.drew.metadata.exif.makernotes.FujifilmMakernoteDirectory
import com.drew.metadata.iptc.IptcDirectory
import com.drew.metadata.xmp.XmpDirectory
import de.huepattl.unterlumen.photo.types.*
import jakarta.enterprise.context.ApplicationScoped
import java.io.File
import java.io.IOException
import java.math.BigDecimal
import java.net.URL
import java.time.LocalDateTime
import java.time.format.DateTimeFormatter

@ApplicationScoped
class ExifMetadataRetriever {

    fun fromFile(filename: String): Metadata {
        val nativeMetadata: com.drew.metadata.Metadata = ImageMetadataReader.readMetadata(File(filename))
        val flatMetadataMap = HashMap<String, String?>()

        nativeMetadata.directories.forEach { dir ->
            //printAll(dir)
            when (dir.name) {
                "JPEG" -> flatMetadataMap.putAll(getJpegInformation(dir))
                "Exif IFD0" -> flatMetadataMap.putAll(getImageBasics(dir))
                "Exif SubIFD" -> flatMetadataMap.putAll(getImageDetails(dir))
                "File Type" -> flatMetadataMap.putAll(getFileTypeDetails(dir))
                "File" -> flatMetadataMap.putAll(getFileDetails(dir))
                "GPS" -> flatMetadataMap.putAll(getGeolocation(dir))
            }
        }

        // get title, description and tags/keywords
        flatMetadataMap.putAll(
            getXmpData(
                nativeMetadata
                    .getFirstDirectoryOfType(IptcDirectory::class.java)
            )
        )

        return buildMetadataFromFLatMap(flatMetadataMap)
    }

    internal fun printAll(dir: Directory) {
        dir.getTags().forEach { tag -> println(tag) }
    }

    internal fun buildMetadataFromFLatMap(map: Map<String, String?>): Metadata {
        var metadata = Metadata(
            title = map["xmp:title"] ?: map["file:name"],
            description = map["xmp:description"],
            tags = map["xmp:tags"]?.split(";").orEmpty(),
            dimensions = Dimensions(
                height = Length.of(map["jpeg:height"]!!),
                width = Length.of(map["jpeg:width"]!!)
            ),
            quality = Quality(
                colourDepth = ColourDepth.of(map["jpeg:data_precision"]!!)
            ),
            cameraBrand = map["ifd0:make"],
            cameraModel = map["ifd0:model"],
            software = map["ifd0:software"],
            artist = map["ifd0:artist"],
            copyright = map["ifd0:copyright"],
            orientation = Orientation.of(map["ifd0:orientation"] ?: "horizontal"),
            exposure = Exposure(
                mode = map["subifd0:exposure_mode"]?.lowercase(),
                meteringMode = map["subifd0:metering_mode"]?.lowercase(),
                time = ExposureTime.of(map["subifd0:exposure_time"] ?: "0 sec"),
                compensation = map["subifd0:exposure_compensation"] ?: "0 EV",
                iso = map["subifd0:iso_speed_ratings"]?.toInt()
            ),
            lens = Lens(
                brand = map["subifd0:lens_make"],
                model = map["subifd0:lens_model"],
                aperture = Aperture.of(map["subifd0:f_number"] ?: "f/8"),
                focalLength = Length.of(map["subifd0:lens_focal_length"] ?: "50 mm"),
            ),
            whiteBalanceMode = map["subifd0:white_balance_mode"]?.lowercase(),
            filename = map["file:name"],
            fileSize = FileSize.of(map["file:size"] ?: "0 byte"),
            mimeType = map["file:mime_type"],
            createdAt = exifDateAndTimeToLocalDateTime(map["subifd0:date_time"] ?: "1970:01:01 00:00:00")
        )

        if (map["geolocation:latitude"] != null && map["geolocation:longitude"] != null && map["geolocation:altitude"] != null) {
            metadata = metadata.copy(
                geoLocation = GeoLocation(
                    latitude = GeoCoordinate.parse(map["geolocation:latitude"] ?: "00° 00' 00,00\""),
                    longitude = GeoCoordinate.parse(map["geolocation:longitude"] ?: "00° 00' 00,00\""),
                    altitude = Length.of(map["geolocation:altitude"] ?: "0 m"),
                )
            )
        }

        return metadata
    }

    internal fun getJpegInformation(dir: Directory): Map<String, String> {
        var map = HashMap<String, String>()
        dir.tags.forEach { tag ->
            when (tag.tagName) {
                "Data Precision" -> map["jpeg:data_precision"] = tag.description // e.g. '8 bits'
                "Image Width" -> map["jpeg:width"] = tag.description // e.g. '1920 pixels'
                "Image Height" -> map["jpeg:height"] = tag.description // e.g. '1280 pixels'
            }
        }
        return map
    }

    internal fun getImageBasics(dir: Directory): Map<String, String> {
        var map = HashMap<String, String>()
        dir.tags.forEach { tag ->
            when (tag.tagName) {
                "Make" -> map["ifd0:make"] = tag.description // e.g. 'FUJIFILM' or 'Canon'
                "Model" -> map["ifd0:model"] = tag.description // e.g. 'X-T50' or 'Canon EOS 200D'
                "Software" -> map["ifd0:software"] = tag.description
                "Orientation" -> map["ifd0:orientation"] =
                    simplifyExifOrientation(dir.getInteger(tag.tagType)) // e.g. 'Horizontal'
                "Artist" -> map["ifd0:artist"] = tag.description
                "Copyright" -> map["ifd0:copyright"] = tag.description
            }
        }
        return map
    }

    internal fun getImageDetails(dir: Directory): Map<String, String> {
        var map = HashMap<String, String>()
        dir.tags.forEach { tag ->
            when (tag.tagName) {
                "Exposure Time" -> map["subifd0:exposure_time"] = tag.description // e.g. '4 sec' or '1/250 sec'
                "Exposure Mode" -> map["subifd0:exposure_mode"] = tag.description
                "Exposure Bias Value" -> map["subifd0:exposure_compensation"] = tag.description // e.g. '-1 EV'
                "Metering Mode" -> map["subifd0:metering_mode"] = tag.description
                "ISO Speed Ratings" -> map["subifd0:iso_speed_ratings"] = tag.description
                "F-Number" -> map["subifd0:f_number"] = tag.description
                "Lens Make" -> map["subifd0:lens_make"] = tag.description
                "Lens Model" -> map["subifd0:lens_model"] = tag.description
                "Focal Length" -> map["subifd0:lens_focal_length"] = tag.description
                "White Balance Mode" -> map["subifd0:white_balance_mode"] = tag.description
                "Date/Time Original" -> map["subifd0:date_time"] = tag.description
            }
        }
        return map
    }

    internal fun getFileDetails(dir: Directory): Map<String, String> {
        var map = HashMap<String, String>()
        dir.tags.forEach { tag ->
            when (tag.tagName) {
                "File Name" -> map["file:name"] = tag.description // e.g. 'DMG00001.jpg'
                "File Size" -> map["file:size"] = tag.description // e.g. '1617709 bytes'
            }
        }
        return map
    }

    internal fun getFileTypeDetails(dir: Directory): Map<String, String> {
        var map = HashMap<String, String>()
        dir.tags.forEach { tag ->
            when (tag.tagName) {
                "Detected MIME Type" -> map["file:mime_type"] = tag.description // e.g. 'image/jpeg'
            }
        }
        return map
    }

    internal fun getGeolocation(dir: Directory): Map<String, String> {
        var map = HashMap<String, String>()
        dir.tags.forEach { tag ->
            when (tag.tagName) {
                "GPS Latitude" -> map["geolocation:latitude"] = tag.description // e.g. '50° 39' 55,06"'
                "GPS Longitude" -> map["geolocation:longitude"] = tag.description // e.g. '7° 12' 35,87"'
                "GPS Altitude" -> map["geolocation:altitude"] = tag.description // e.g. '329,48 metres'
            }
        }
        return map
    }

    internal fun getXmpData(dir: IptcDirectory?): Map<String, String> {
        var map = HashMap<String, String>()
        map["xmp:title"] = dir?.getString(IptcDirectory.TAG_OBJECT_NAME) ?: ""
        map["xmp:description"] = dir?.getString(IptcDirectory.TAG_CAPTION) ?: ""
        map["xmp:tags"] = dir?.getStringArray(IptcDirectory.TAG_KEYWORDS)?.joinToString(";") ?: ""
        return map
    }

    /**
     * ExIF distinguishes orientation in rotation directions (clockwise etc.) and mirroring. For us, it is just
     * fine to know if we are dealing with a more horizontal or vertical image.
     *
     * @param exifOrientation integer representation of the orientation tag
     * @return string representation 'horizontal', 'vertical' or 'unknown'
     */
    internal fun simplifyExifOrientation(exifOrientation: Int): String {
        when (exifOrientation) {
            1, 2, 3, 4 -> return "horizontal"
            5, 6, 7, 8 -> return "vertical"
            else -> return "unknown"
        }
    }

    /**
     * Parses an EXIF date and time and returns it as a local date. We need this since especially
     * a date string in EXIF have a strange format - they use colons instead of dashes like so: "2025:02:01"
     */
    internal fun exifDateAndTimeToLocalDateTime(exifDateAndTime: String): LocalDateTime {
        val formatter = DateTimeFormatter.ofPattern("yyyy:MM:dd HH:mm:ss")
        val exifDateStr = exifDateAndTime
        return LocalDateTime.parse(exifDateStr, formatter)
    }

}