package coinselect

import (
	"errors"
	"math"
	"sort"
)

var (
	ErrInvalidRequest    = errors.New("coinselect: values, sizes and fee rate must be positive")
	ErrInsufficientFunds = errors.New("coinselect: insufficient funds")
	ErrOverflow          = errors.New("coinselect: integer overflow")
)

type UTXO struct {
	ID          string
	Value       int64
	InputVBytes int64
}

type Request struct {
	UTXOs         []UTXO
	Target        int64
	FeeRate       int64
	BaseVBytes    int64
	ChangeVBytes  int64
	DustThreshold int64
}

type Selection struct {
	UTXOs  []UTXO
	Total  int64
	Fee    int64
	Change int64
}

// LargestFirst is a teaching implementation, not a production wallet policy.
// It demonstrates value conservation, input-dependent fees and dust handling.
func LargestFirst(request Request) (Selection, error) {
	if request.Target <= 0 || request.FeeRate <= 0 || request.BaseVBytes <= 0 ||
		request.ChangeVBytes <= 0 || request.DustThreshold < 0 {
		return Selection{}, ErrInvalidRequest
	}

	candidates := append([]UTXO(nil), request.UTXOs...)
	for _, candidate := range candidates {
		if candidate.Value <= 0 || candidate.InputVBytes <= 0 {
			return Selection{}, ErrInvalidRequest
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Value == candidates[j].Value {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Value > candidates[j].Value
	})

	var selected []UTXO
	var total, inputVBytes int64
	for _, candidate := range candidates {
		var err error
		total, err = safeAdd(total, candidate.Value)
		if err != nil {
			return Selection{}, err
		}
		inputVBytes, err = safeAdd(inputVBytes, candidate.InputVBytes)
		if err != nil {
			return Selection{}, err
		}
		selected = append(selected, candidate)

		noChangeSize, err := safeAdd(request.BaseVBytes, inputVBytes)
		if err != nil {
			return Selection{}, err
		}
		minimumFee, err := safeMul(noChangeSize, request.FeeRate)
		if err != nil {
			return Selection{}, err
		}
		required, err := safeAdd(request.Target, minimumFee)
		if err != nil {
			return Selection{}, err
		}
		if total < required {
			continue
		}

		withChangeSize, err := safeAdd(noChangeSize, request.ChangeVBytes)
		if err != nil {
			return Selection{}, err
		}
		withChangeFee, err := safeMul(withChangeSize, request.FeeRate)
		if err != nil {
			return Selection{}, err
		}
		change := total - request.Target - withChangeFee
		if change >= request.DustThreshold {
			return Selection{
				UTXOs:  selected,
				Total:  total,
				Fee:    withChangeFee,
				Change: change,
			}, nil
		}

		// A non-economic remainder is added to the miner fee by omitting change.
		return Selection{
			UTXOs: selected,
			Total: total,
			Fee:   total - request.Target,
		}, nil
	}

	return Selection{}, ErrInsufficientFunds
}

func safeAdd(left, right int64) (int64, error) {
	if right > 0 && left > math.MaxInt64-right {
		return 0, ErrOverflow
	}
	return left + right, nil
}

func safeMul(left, right int64) (int64, error) {
	if left != 0 && right > math.MaxInt64/left {
		return 0, ErrOverflow
	}
	return left * right, nil
}
