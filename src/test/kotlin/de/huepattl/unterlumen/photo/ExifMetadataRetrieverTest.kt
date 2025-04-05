package de.huepattl.unterlumen.photo

import de.huepattl.unterlumen.photo.types.ColourDepth
import de.huepattl.unterlumen.photo.types.FileSize
import de.huepattl.unterlumen.photo.types.Length
import de.huepattl.unterlumen.photo.types.Orientation
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import java.math.BigDecimal
import java.time.LocalDateTime

class ExifMetadataRetrieverTest {

    @Test
    fun `test ExIF JPEG Fujifilm X-T50 reference test`() {
        // given
        val imageFile = "images/fujifilm_x-t50_bird.jpeg" // was _DSF1806.jpeg
        val resourceUrl = javaClass.classLoader.getResource(imageFile)
            ?: throw IllegalArgumentException("resource not found: $imageFile")

        // when
        val metadata = ExifMetadataRetriever().fromFile(resourceUrl.path)

        // then
        assertEquals("Mönchsgrasmücke auf Ast", metadata.title)
        assertEquals("Bild einer Mönchsgrasmücke", metadata.description)
        assertEquals(listOf("Wildlife", "Mönchsgrasmücke", "Bird", "Eurasian Blackcap"), metadata.tags)
        assertEquals(2771, metadata.dimensions?.width?.value)
        assertEquals(Length.Unit.PIXELS, metadata.dimensions?.width?.unit)

        assertEquals(2649, metadata.dimensions?.height?.value)
        assertEquals(Length.Unit.PIXELS, metadata.dimensions?.height?.unit)

        assertEquals(8, metadata.quality?.colourDepth?.value)
        assertEquals(ColourDepth.Unit.BIT, metadata.quality?.colourDepth?.unit)

        assertEquals("FUJIFILM", metadata.cameraBrand)
        assertEquals("X-T50", metadata.cameraModel)
        assertEquals("Digital Camera X-T50 Ver1.12", metadata.software)

        assertEquals(Orientation.HORIZONTAL, metadata.orientation)

        assertEquals("1.33 EV", metadata.exposure?.compensation?.toString())
        assertEquals("1/900 sec", metadata.exposure?.time?.toString())
        assertEquals("auto exposure", metadata.exposure?.mode)
        assertEquals("multi-segment", metadata.exposure?.meteringMode)
        assertEquals(2500, metadata.exposure?.iso)

        assertEquals("FUJIFILM", metadata.lens?.brand)
        assertEquals("XF70-300mmF4-5.6 R LM OIS WR", metadata.lens?.model)
        assertEquals("f/5.6", metadata.lens?.aperture.toString())
        assertEquals(300, metadata.lens?.focalLength?.value)
        assertEquals(Length.Unit.MILLIMETERS, metadata.lens?.focalLength?.unit)

        assertEquals("auto white balance", metadata.whiteBalanceMode)

        assertEquals("fujifilm_x-t50_bird.jpeg", metadata.filename)
        assertEquals(1898021, metadata.fileSize?.value)
        assertEquals(FileSize.Unit.BYTE, metadata.fileSize?.unit)
        assertEquals("image/jpeg", metadata.mimeType)

        assertEquals(LocalDateTime.of(2025, 4, 2, 12, 54, 58), metadata.createdAt)
    }

    @Test
    fun `iPhone test geolocation`() {
        // given
        val imageFile = "images/apple_iphone6s_drachenfels-geolocation.jpeg"
        val resourceUrl = javaClass.classLoader.getResource(imageFile)
            ?: throw IllegalArgumentException("resource not found: $imageFile")

        // when
        val metadata = ExifMetadataRetriever().fromFile(resourceUrl.path)

        // then
        assertEquals(4032, metadata.dimensions?.width?.value)

        assertEquals(50, metadata.geoLocation?.latitude?.degrees)
        assertEquals(39, metadata.geoLocation?.latitude?.minutes)
        assertEquals(BigDecimal("55.06"), metadata.geoLocation?.latitude?.seconds)

        assertEquals(7, metadata.geoLocation?.longitude?.degrees)
        assertEquals(12, metadata.geoLocation?.longitude?.minutes)
        assertEquals(BigDecimal("35.87"), metadata.geoLocation?.longitude?.seconds)

        assertEquals(329, metadata.geoLocation?.altitude?.value)
        assertEquals(Length.Unit.METERS, metadata.geoLocation?.altitude?.unit)
    }

}