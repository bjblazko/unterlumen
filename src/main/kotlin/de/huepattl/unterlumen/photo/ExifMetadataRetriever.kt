package de.huepattl.unterlumen.photo

import com.drew.imaging.ImageMetadataReader
import com.drew.metadata.Directory
import de.huepattl.unterlumen.photo.types.*
import java.io.File

class ExifMetadataRetriever {

    fun fromFile(filename: String): Metadata {
        val nativeMetadata: com.drew.metadata.Metadata = ImageMetadataReader.readMetadata(File(filename))
        val flatMetadataMap = HashMap<String, String?>()

        nativeMetadata.directories.forEach { dir ->
            when (dir.name) {
                "JPEG" -> flatMetadataMap.putAll(getJpegInformation(dir))
                "Exif IFD0" -> flatMetadataMap.putAll(getImageBasics(dir))
                "Exif SubIFD" -> flatMetadataMap.putAll(getImageDetails(dir))
                "IPTC" -> {} // TODO
                "File Type" -> {}
                "File" -> {} // TODO
            }
        }

        return buildMetadataFromFLatMap(flatMetadataMap)
    }

    internal fun buildMetadataFromFLatMap(map: Map<String, String?>): Metadata {
        return Metadata(
            dimensions = Dimensions(
                height = Length.of(map["jpeg_height"]!!),
                width = Length.of(map["jpeg_width"]!!)
            ),
            quality = Quality(
                colourDepth = ColourDepth.of(map["jpeg_data_precision"]!!)
            ),
            cameraBrand = map["ifd0_make"],
            cameraModel = map["ifd0_model"],
            software = map["ifd0_software"],
            artist = map["ifd0_artist"],
            copyright = map["ifd0_copyright"],
            orientation = Orientation.of(map["ifd0_orientation"]!!),
            exposure = Exposure(
                mode = map["subifd0_exposure_mode"]?.lowercase(),
                meteringMode = map["subifd0_metering_mode"]?.lowercase(),
                time = ExposureTime.of(map["subifd0_exposure_time"]!!),
                compensation = ExposureCompensation.of(map["subifd0_exposure_compensation"]!!),
                iso = map["subifd0_iso_speed_ratings"]?.toInt()
            ),
            lens = Lens(
                brand = map["subifd0_lens_make"],
                model = map["subifd0_lens_model"],
                aperture = Aperture.of(map["subifd0_f_number"]!!),
                focalLength = Length.of(map["subifd0_lens_focal_length"]!!),
            )
        )
    }

    internal fun getJpegInformation(dir: Directory): Map<String, String> {
        var map = HashMap<String, String>()
        dir.tags.forEach { tag ->
            when (tag.tagName) {
                "Data Precision" -> map["jpeg_data_precision"] = tag.description // e.g. '8 bits'
                "Image Width" -> map["jpeg_width"] = tag.description // e.g. '1920 pixels'
                "Image Height" -> map["jpeg_height"] = tag.description // e.g. '1280 pixels'
            }
        }
        return map
    }

    internal fun getImageBasics(dir: Directory): Map<String, String> {
        var map = HashMap<String, String>()
        dir.tags.forEach { tag ->
            when (tag.tagName) {
                "Make" -> map["ifd0_make"] = tag.description // e.g. 'FUJIFILM' or 'Canon'
                "Model" -> map["ifd0_model"] = tag.description // e.g. 'X-T50' or 'Canon EOS 200D'
                "Software" -> map["ifd0_software"] = tag.description
                "Orientation" -> map["ifd0_orientation"] =
                    simplifyExifOrientation(dir.getInteger(tag.tagType)) // e.g. 'Horizontal'
                "Artist" -> map["ifd0_artist"] = tag.description
                "Copyright" -> map["ifd0_copyright"] = tag.description
            }
        }
        return map
    }

    internal fun getImageDetails(dir: Directory): Map<String, String> {
        var map = HashMap<String, String>()
        dir.tags.forEach { tag ->
            when (tag.tagName) {
                "Exposure Time" -> map["subifd0_exposure_time"] = tag.description // e.g. '4 sec' or '1/250 sec'
                "Exposure Mode" -> map["subifd0_exposure_mode"] = tag.description
                "Exposure Bias Value" -> map["subifd0_exposure_compensation"] = tag.description // e.g. '-1 EV'
                "Metering Mode" -> map["subifd0_metering_mode"] = tag.description
                "ISO Speed Ratings" -> map["subifd0_iso_speed_ratings"] = tag.description
                "F-Number" -> map["subifd0_f_number"] = tag.description
                "Lens Make" -> map["subifd0_lens_make"] = tag.description
                "Lens Model" -> map["subifd0_lens_model"] = tag.description
                "Focal Length" -> map["subifd0_lens_focal_length"] = tag.description
            }
        }
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

}