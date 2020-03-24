package daemon

import (
	"github.com/NavExplorer/navexplorer-indexer-go/generated/dic"
	"github.com/NavExplorer/navexplorer-indexer-go/internal/config"
	"github.com/NavExplorer/navexplorer-indexer-go/internal/indexer"
	"github.com/getsentry/raven-go"
	"github.com/sarulabs/dingo/v3"
	log "github.com/sirupsen/logrus"
)

var container *dic.Container

func Execute() {
	config.Init()

	container, _ = dic.NewContainer(dingo.App)
	container.GetElastic().InstallMappings()
	container.GetSoftforkService().LoadSoftForks()

	if config.Get().Sentry.Active {
		_ = raven.SetDSN(config.Get().Sentry.DSN)
	}
	indexer.LastBlockIndexed = getHeight()

	log.Infof("Rewind from %d to %d", indexer.LastBlockIndexed+uint64(config.Get().BulkIndexSize), indexer.LastBlockIndexed)
	if err := container.GetRewinder().RewindToHeight(indexer.LastBlockIndexed); err != nil {
		log.WithError(err).Fatal("Failed to rewind index")
	}

	if indexer.LastBlockIndexed != 0 {
		if block, err := container.GetBlockRepo().GetBlockByHeight(indexer.LastBlockIndexed); err != nil {
			log.WithError(err).Fatal("Failed to get block at height: ", indexer.LastBlockIndexed)
		} else {
			consensus, err := container.GetDaoConsensusRepo().GetConsensus()
			if err == nil {
				blockCycle := block.BlockCycle(consensus.BlocksPerVotingCycle, consensus.MinSumVotesPerVotingCycle)
				container.GetDaoProposalService().LoadVotingProposals(block, blockCycle)
				container.GetDaoPaymentRequestService().LoadVotingPaymentRequests(block, blockCycle)
			}
		}
	}

	container.GetIndexer().BulkIndex()
	container.GetSubscriber().Subscribe()
}

func getHeight() uint64 {
	if height, err := container.GetBlockRepo().GetHeight(); err != nil {
		log.WithError(err).Fatal("Failed to get block height")
	} else {
		if height >= uint64(config.Get().BulkIndexSize) {
			return height - uint64(config.Get().BulkIndexSize)
		}
	}

	return 0
}
