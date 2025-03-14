package de.huepattl.unterlumen.photo

import de.huepattl.unterlumen.photo.types.ColourDepth
import de.huepattl.unterlumen.photo.types.Length
import de.huepattl.unterlumen.photo.types.Orientation
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test

class ExifMetadataRetrieverTest {

    @Test
    fun `test ExIF JPEG Fujifilm X-T50 reference test`() {
        // given
        val imageFile = "images/fujifilm_x-t50_bird.jpeg"
        val resourceUrl = javaClass.classLoader.getResource(imageFile)
            ?: throw IllegalArgumentException("resource not found: $imageFile")

        // when
        val metadata = ExifMetadataRetriever().fromFile(resourceUrl.path)

        // then
        assertEquals(1920, metadata.dimensions?.width?.value)
        assertEquals(Length.Unit.PIXELS, metadata.dimensions?.width?.unit)

        assertEquals(1280, metadata.dimensions?.height?.value)
        assertEquals(Length.Unit.PIXELS, metadata.dimensions?.height?.unit)

        assertEquals(8, metadata.quality?.colourDepth?.value)
        assertEquals(ColourDepth.Unit.BIT, metadata.quality?.colourDepth?.unit)

        assertEquals("FUJIFILM", metadata.cameraBrand)
        assertEquals("X-T50", metadata.cameraModel)
        assertEquals("Digital Camera X-T50 Ver1.10", metadata.software)

        assertEquals(Orientation.HORIZONTAL, metadata.orientation)

        assertEquals("0 EV", metadata.exposure?.compensation?.toString())
        assertEquals("1/900 sec", metadata.exposure?.time?.toString())
        assertEquals("auto exposure", metadata.exposure?.mode)
        assertEquals("multi-segment", metadata.exposure?.meteringMode)
        assertEquals(500, metadata.exposure?.iso)
    }

}