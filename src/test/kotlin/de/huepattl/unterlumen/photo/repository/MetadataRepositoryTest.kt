package de.huepattl.unterlumen.photo.repository

import de.huepattl.unterlumen.photo.types.ColourDepth
import de.huepattl.unterlumen.photo.types.ExposureTime
import de.huepattl.unterlumen.photo.types.Length
import io.quarkus.test.junit.QuarkusTest
import jakarta.inject.Inject
import jakarta.transaction.Transactional
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Test
import java.math.BigDecimal
import java.time.LocalDateTime

@QuarkusTest
@Transactional
class MetadataRepositoryTest {

    @Inject
    lateinit var repository: MetadataRepository

    var metadataEntity: MetadataEntity = MetadataEntity()

    init {
        metadataEntity.apply {
            this.title = "Nice picture!"
            description = "Photo showing the USS Enterprise NCC 1701-D"
            tags = "galaxy class, enterprise, federation"
            width = 1024
            height = 800
            cameraBrand = "Starfleet"
            cameraModel = "Utopia Planitia Fleet Yards Mars"
            software = "LCARS"
            copyright = "Dr Lea Brahms"
            artist = "Andrew Probert"
            whiteBalanceMode = "auto"
            orientation = "horizontal"
            look = "Classic Chrome"
            quality = 8
            qualityUnit = "bit"
            exposure = ExposureEntity().apply {
                iso = 125
                exposureMode = "auto"
                meteringMode = "matrix"
                timeNumerator = 1
                timeDenominator = 250
                compensation = "-1 EV"
            }
            lensSettingsId = LensSettingsEntity().apply {
                brand = "Sirius Cybernetics Corporation"
                model = "Orion 20-70mm"
                focalLength = 56
                aperture = BigDecimal("1.2")
            }
            geolocation = GeolocationEntity().apply {
                latitudeDegrees = 10
                latitudeMinutes = 20
                latitudeSeconds = BigDecimal("30.40")
                longitudeDegrees = 11
                longitudeMinutes = 21
                longitudeSeconds = BigDecimal("31.41")
                altitude = 3_000
            }
            sizeInBytes = 1701
            photoCreatedAt = LocalDateTime.now()
            mimeType = "image/jpeg"
            filename = "NCC-1701-D-dorsal.jpeg"
        }

    }


    @Test
    fun `map entity to domain metadata object`() {
        // given
        val entity = metadataEntity

        // when
        val metadata = entity.toDomainObject()

        // then
        assertNotNull(metadata)
        assertEquals(entity.id, metadata.id.toString())
        assertEquals(entity.title, metadata.title)
        assertEquals(entity.description, metadata.description)
        assertEquals(entity.tags?.split(","), metadata.tags)
        assertEquals(entity.cameraBrand, metadata.cameraBrand)
        assertEquals(entity.cameraModel, metadata.cameraModel)
        assertEquals(entity.software, metadata.software)
        assertEquals(entity.copyright, metadata.copyright)
        assertEquals(entity.artist, metadata.artist)

        assertEquals(entity.width, metadata.dimensions?.width?.value)
        assertEquals(Length.Unit.PIXELS, metadata.dimensions?.width?.unit)
        assertEquals(entity.height, metadata.dimensions?.height?.value)
        assertEquals(Length.Unit.PIXELS, metadata.dimensions?.height?.unit)

        assertEquals(entity.orientation, metadata.orientation?.name?.lowercase())
        assertEquals(entity.whiteBalanceMode, metadata.whiteBalanceMode)
        assertEquals(entity.look, metadata.look)

        assertEquals(entity.quality, metadata.quality?.colourDepth?.value)
        assertEquals(ColourDepth.Unit.BIT, metadata.quality?.colourDepth?.unit)

        assertEquals(entity.exposure?.iso, metadata.exposure?.iso)
        assertEquals(entity.exposure?.exposureMode, metadata.exposure?.mode)
        assertEquals(entity.exposure?.meteringMode, metadata.exposure?.meteringMode)
        assertEquals(entity.exposure?.timeNumerator, metadata.exposure?.time?.value?.numerator)
        assertEquals(entity.exposure?.timeDenominator, metadata.exposure?.time?.value?.denominator)
        assertEquals(ExposureTime.Unit.SECONDS, metadata.exposure?.time?.unit)

        assertEquals(entity.lensSettingsId?.brand, metadata.lens?.brand)
        assertEquals(entity.lensSettingsId?.model, metadata.lens?.model)
        assertEquals(entity.lensSettingsId?.aperture, metadata.lens?.aperture?.value)
        assertEquals(entity.lensSettingsId?.focalLength, metadata.lens?.focalLength?.value)
        assertEquals(Length.Unit.MILLIMETERS, metadata.lens?.focalLength?.unit)

        assertEquals(entity.geolocation?.latitudeDegrees, metadata.geoLocation?.latitude?.degrees)
        assertEquals(entity.geolocation?.latitudeMinutes, metadata.geoLocation?.latitude?.minutes)
        assertEquals(entity.geolocation?.latitudeSeconds, metadata.geoLocation?.latitude?.seconds)
        assertEquals(entity.geolocation?.longitudeDegrees, metadata.geoLocation?.longitude?.degrees)
        assertEquals(entity.geolocation?.longitudeMinutes, metadata.geoLocation?.longitude?.minutes)
        assertEquals(entity.geolocation?.longitudeSeconds, metadata.geoLocation?.longitude?.seconds)
        assertEquals(entity.geolocation?.altitude, metadata.geoLocation?.altitude?.value)
        assertEquals(Length.Unit.METERS, metadata.geoLocation?.altitude?.unit)

        assertEquals(entity.mimeType, metadata.mimeType)
        assertEquals(entity.filename, metadata.filename)
        assertEquals(entity.photoCreatedAt, metadata.createdAt)
        assertEquals(entity.sizeInBytes, metadata.fileSize)
    }

    @Test
    fun `create and read metadata works`() {
        repository.persist(metadataEntity)

        val found = repository.findById(metadataEntity.id!!)

        assertNotNull(found)
        assertEquals(metadataEntity, found!!)
        // TODO other attributes
    }

}